package httpadapter

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// statusRecorder es un decorador de ResponseWriter cuya única responsabilidad
// es capturar el código de estado HTTP devuelto por los handlers.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Metrics encapsula los colectores Prometheus para el tráfico HTTP
type Metrics struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestsInFlight prometheus.Gauge
}

// NewMetrics inicializa y registra los colectores en el registro de Prometheus
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total de peticiones HTTP procesadas por el microservicio",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duración en segundos de las peticiones HTTP procesadas",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		requestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Número actual de peticiones HTTP en ejecución activa",
			},
		),
	}

	reg.MustRegister(m.requestsTotal, m.requestDuration, m.requestsInFlight)
	return m
}

// Funciones atómicas (Responsabilidad Única)

func (m *Metrics) incInFlight() {
	m.requestsInFlight.Inc()
}

func (m *Metrics) decInFlight() {
	m.requestsInFlight.Dec()
}

func (m *Metrics) observeDuration(method, path string, start time.Time) {
	duration := time.Since(start).Seconds()
	m.requestDuration.WithLabelValues(method, path).Observe(duration)
}

func (m *Metrics) countRequest(method, path string, statusCode int) {
	statusStr := strconv.Itoa(statusCode)
	m.requestsTotal.WithLabelValues(method, path, statusStr).Inc()
}

// Middleware es la función orquestadora de alto nivel que coordina las mediciones atómicas
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Excluir el propio endpoint de scraping para no sesgar las métricas del negocio
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		recorder := newStatusRecorder(w)
		m.incInFlight()
		defer m.decInFlight()

		start := time.Now()
		next.ServeHTTP(recorder, r)

		m.observeDuration(r.Method, r.URL.Path, start)
		m.countRequest(r.Method, r.URL.Path, recorder.statusCode)
	})
}
