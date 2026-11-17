// soak_test.js - Orquestador de la Prueba de Resistencia y Durabilidad (Soak / Endurance Test)
// Diseñado para ejecutarse durante períodos prolongados (30m, 1h, 2h) contra Kubernetes (AKS)
// Evalúa fugas de memoria (memory leaks), retención en el pool de PostgreSQL y estabilidad del GC

export { soakProfile as options } from './config.js';
export { runUserRegistrationVU as default } from './runner.js';
