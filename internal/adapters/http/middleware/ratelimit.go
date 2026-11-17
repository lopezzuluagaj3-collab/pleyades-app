package middleware

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
}

// RateLimiter implementa el algoritmo Token Bucket por dirección IP
type RateLimiter struct {
	mu         sync.Mutex
	clients    map[string]*clientBucket
	rate       float64       // Tokens añadidos por segundo
	burst      float64       // Capacidad máxima de ráfaga
	cleanupTTL time.Duration // Tiempo de inactividad antes de purgar una IP
}

// NewRateLimiter inicializa un limitador de tasa Token Bucket.
// ratePerSecond: número de peticiones permitidas por segundo.
// burst: ráfaga máxima permitida.
// cleanupTTL: tiempo tras el cual se liberan las IPs inactivas para evitar fugas de memoria.
func NewRateLimiter(ratePerSecond, burst float64, cleanupTTL time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:    make(map[string]*clientBucket),
		rate:       ratePerSecond,
		burst:      burst,
		cleanupTTL: cleanupTTL,
	}

	// Goroutine periódica para limpieza de IPs inactivas (previene memory leak)
	go rl.startCleanupLoop(cleanupTTL)

	return rl
}

// Allow evalúa de forma atómica y thread-safe si una IP tiene tokens disponibles
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.clients[ip]

	if !exists {
		// Cliente nuevo: bucket lleno, consume 1 token
		rl.clients[ip] = &clientBucket{
			tokens:     rl.burst - 1,
			lastRefill: now,
		}
		return true
	}

	// Calcular tokens acumulados por el paso del tiempo
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens = math.Min(rl.burst, bucket.tokens+(elapsed*rl.rate))
	bucket.lastRefill = now

	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// Middleware orquesta la inspección de la IP y la decisión de admitir o rechazar (429)
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)

		if !rl.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "límite de peticiones excedido, intente de nuevo en unos segundos",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractClientIP(r *http.Request) string {
	// 1. Revisar X-Forwarded-For (típico cuando hay un Ingress Controller o Reverse Proxy en K8s)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(ips[0])
		if clientIP != "" {
			return clientIP
		}
	}

	// 2. Revisar X-Real-IP
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// 3. Fallback a RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	return r.RemoteAddr
}

func (rl *RateLimiter) startCleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for now := range ticker.C {
		rl.mu.Lock()
		for ip, bucket := range rl.clients {
			if now.Sub(bucket.lastRefill) > rl.cleanupTTL {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}
