// spike_test.js - Orquestador de la Prueba de Picos Abruptos (Spike Test)
// Evalúa la tolerancia a ráfagas violentas de tráfico sobre el Ingress Controller y el socket buffer

export { spikeProfile as options } from './config.js';
export { runUserRegistrationVU as default } from './runner.js';
