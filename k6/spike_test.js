// spike_test.js - Orquestador de la Prueba de Picos Abruptos (Spike Test)
// Evalúa la tolerancia a ráfagas violentas de tráfico sobre el Ingress Controller y el socket buffer

import { spikeProfile } from './config.js';
import { runUserRegistrationVU } from './runner.js';

export const options = spikeProfile;
export default runUserRegistrationVU;
