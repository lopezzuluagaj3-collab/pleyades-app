package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type idempotencyStatus int

const (
	statusInFlight idempotencyStatus = iota
	statusCompleted
)

type cachedResponse struct {
	status     idempotencyStatus
	statusCode int
	headers    http.Header
	body       []byte
	createdAt  time.Time
}

// responseCapture intercepta la escritura de la respuesta para almacenarla en caché
type responseCapture struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func newResponseCapture(w http.ResponseWriter) *responseCapture {
	return &responseCapture{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

func (r *responseCapture) WriteHeader(code int) {
	r.statusCode = code
}

func (r *responseCapture) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

// IdempotencyMiddleware garantiza que peticiones con el encabezado "Idempotency-Key"
// no produzcan mutaciones duplicadas ante reintentos de red.
type IdempotencyMiddleware struct {
	mu         sync.RWMutex
	cache      map[string]*cachedResponse
	cleanupTTL time.Duration
}

// NewIdempotencyMiddleware inicializa el middleware de idempotencia con purga periódica
func NewIdempotencyMiddleware(cleanupTTL time.Duration) *IdempotencyMiddleware {
	im := &IdempotencyMiddleware{
		cache:      make(map[string]*cachedResponse),
		cleanupTTL: cleanupTTL,
	}

	go im.startCleanupLoop(cleanupTTL)
	return im
}

// Middleware intercepta peticiones con cabecera Idempotency-Key
func (m *IdempotencyMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idempotencyKey := r.Header.Get("Idempotency-Key")

		// Si no se envía clave de idempotencia, se procesa normalmente
		if idempotencyKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		m.mu.Lock()
		cached, exists := m.cache[idempotencyKey]

		if exists {
			// Caso 1: Petición idéntica aún en ejecución activa en otra goroutine -> 409 Conflict
			if cached.status == statusInFlight {
				m.mu.Unlock()
				respondIdempotencyError(w, http.StatusConflict, "petición en curso con la misma clave de idempotencia")
				return
			}

			// Caso 2: Petición completada previamente -> Repetir respuesta exacta desde memoria
			m.mu.Unlock()
			for k, values := range cached.headers {
				for _, v := range values {
					w.Header().Add(k, v)
				}
			}
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(cached.statusCode)
			_, _ = w.Write(cached.body)
			return
		}

		// Caso 3: Clave nueva -> Marcar como En Curso (In-Flight)
		m.cache[idempotencyKey] = &cachedResponse{
			status:    statusInFlight,
			createdAt: time.Now(),
		}
		m.mu.Unlock()

		// Interceptar la respuesta del handler
		capture := newResponseCapture(w)
		next.ServeHTTP(capture, r)

		// Guardar en caché el resultado final completado
		m.mu.Lock()
		m.cache[idempotencyKey] = &cachedResponse{
			status:     statusCompleted,
			statusCode: capture.statusCode,
			headers:    capture.ResponseWriter.Header().Clone(),
			body:       capture.body.Bytes(),
			createdAt:  time.Now(),
		}
		m.mu.Unlock()

		// Transmitir la respuesta original al cliente
		capture.ResponseWriter.WriteHeader(capture.statusCode)
		_, _ = capture.ResponseWriter.Write(capture.body.Bytes())
	})
}

func respondIdempotencyError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (m *IdempotencyMiddleware) startCleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for now := range ticker.C {
		m.mu.Lock()
		for key, item := range m.cache {
			if now.Sub(item.createdAt) > m.cleanupTTL {
				delete(m.cache, key)
			}
		}
		m.mu.Unlock()
	}
}
