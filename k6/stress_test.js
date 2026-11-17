// stress_test.js - Orquestador de la Prueba de Estrés (Stress Test)
// Evalúa el comportamiento del microservicio en su límite superior y el autoescalado HPA en Kubernetes

export { stressProfile as options } from './config.js';
export { runUserRegistrationVU as default } from './runner.js';
