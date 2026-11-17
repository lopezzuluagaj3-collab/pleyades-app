// soak_test.js - Orquestador de la Prueba de Resistencia y Durabilidad (Soak / Endurance Test)
// Diseñado para ejecutarse durante períodos prolongados (30m, 1h, 2h) contra Kubernetes (AKS)
// Evalúa fugas de memoria (memory leaks), retención en el pool de PostgreSQL y estabilidad del GC

import { soakProfile } from './config.js';
import { runUserRegistrationVU } from './runner.js';

export const options = soakProfile;
export default runUserRegistrationVU;
