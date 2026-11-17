// stress_test.js - Orquestador de la Prueba de Estrés (Stress Test)
// Evalúa el comportamiento del microservicio en su límite superior y el autoescalado HPA en Kubernetes

import { stressProfile } from './config.js';
import { runUserRegistrationVU } from './runner.js';

export const options = stressProfile;
export default runUserRegistrationVU;
