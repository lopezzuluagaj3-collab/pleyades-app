// config.js - Responsabilidad Única: Definir configuración de entorno, perfiles de carga y SLOs (Thresholds)

export const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

// Perfil de carga progresiva (Ramping VUs)
export const loadProfile = {
  stages: [
    { duration: '20s', target: 10 }, // Ramp-up: Calentamiento de 0 a 10 usuarios concurrentes
    { duration: '40s', target: 25 }, // Carga sostenida: 25 usuarios concurrentes
    { duration: '20s', target: 50 }, // Pico de estrés: 50 usuarios concurrentes
    { duration: '20s', target: 0 },  // Ramp-down: Enfriamiento a 0 usuarios
  ],
  thresholds: {
    // SLO de Latencia: el 95% de las peticiones debe responder en menos de 250ms
    http_req_duration: ['p(95)<250', 'p(99)<500'],
    // SLO de Disponibilidad: menos del 1% de fallos permitidos
    http_req_failed: ['rate<0.01'],
  },
};
