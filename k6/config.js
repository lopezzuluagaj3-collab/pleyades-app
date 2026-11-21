// config.js - Responsabilidad Única: Definir configuración de entorno, perfiles de carga, estrés, resistencia y SLOs

export const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';

// 1. Perfil de Carga Estándar (Load Test)
// Simula tráfico diario típico y picos previstos de operación
export const loadProfile = {
  stages: [
    { duration: '20s', target: 10 }, // Calentamiento (Ramp-up)
    { duration: '40s', target: 25 }, // Carga sostenida
    { duration: '20s', target: 50 }, // Pico moderado
    { duration: '20s', target: 0 },  // Enfriamiento (Ramp-down)
  ],
  thresholds: {
    // Modelo de Capacidad vs. SLO para operaciones intensivas en CPU (Bcrypt cost=10 ~70ms por hash):
    // Con 2 vCPUs regionales asignadas a la app en AKS (~28 hashes/s de capacidad física saturada),
    // a 50 VUs concurrentes en registro de usuarios la latencia p95 bajo saturación tiene SLO < 1500ms.
    http_req_duration: ['p(95)<1500', 'p(99)<2500'],
    http_req_failed: ['rate<0.01'], // Menos del 1% de errores
  },
};

// 2. Perfil de Estrés (Stress Test)
// Empuja el sistema hacia su punto de quiebre (Breaking Point) y evalúa el autoescalado HPA en Kubernetes
export const stressProfile = {
  stages: [
    { duration: '30s', target: 25 },  // Calentamiento rápido
    { duration: '1m', target: 50 },   // Carga base alta
    { duration: '1m', target: 100 },  // Carga pesada (Inicio de presión en CPU y Pool DB)
    { duration: '1m', target: 200 },  // Estrés extremo (Debe disparar réplicas del HPA en K8s)
    { duration: '30s', target: 300 }, // Punto de quiebre / Saturación máxima
    { duration: '1m', target: 0 },    // Recuperación del clúster (Cool-down)
  ],
  thresholds: {
    // En estrés extremo se tolera una degradación controlada mientras los nuevos pods inician
    http_req_duration: ['p(95)<1000'], // Latencia p95 bajo 1s durante estrés
    http_req_failed: ['rate<0.05'],    // Tasa de fallo inferior al 5%
  },
};

// 3. Perfil de Resistencia / Durabilidad (Soak / Endurance Test)
// Carga continua moderada durante períodos prolongados para detectar fugas de memoria (memory leaks),
// retención de conexiones en el pool de PostgreSQL y acumulación de basura en el Garbage Collector (GC).
const soakDuration = __ENV.SOAK_DURATION || '30m';
const soakVUs = Number.parseInt(__ENV.SOAK_VUS || '20', 10);

export const soakProfile = {
  stages: [
    { duration: '2m', target: soakVUs },       // Subida suave a la carga nominal
    { duration: soakDuration, target: soakVUs },// Carga sostenida continua (30m, 1h, 2h...)
    { duration: '2m', target: 0 },              // Descenso controlado
  ],
  thresholds: {
    // La latencia no debe degradarse con el paso de las horas (síntoma de memory leak o saturación del pool DB)
    http_req_duration: ['p(95)<1500', 'p(99)<2500'],
    http_req_failed: ['rate<0.01'],
  },
};

// 4. Perfil de Picos Repentinos (Spike Test)
// Evalúa la resiliencia del Ingress Controller y el microservicio ante avalanchas instantáneas de tráfico
export const spikeProfile = {
  stages: [
    { duration: '10s', target: 10 },  // Tráfico base mínimo
    { duration: '10s', target: 200 }, // Disparo instantáneo de tráfico (Spike agresivo)
    { duration: '30s', target: 200 }, // Meseta corta de saturación
    { duration: '10s', target: 0 },   // Caída abrupta
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'], // Tolerancia máxima del 5% durante el impacto
  },
};
