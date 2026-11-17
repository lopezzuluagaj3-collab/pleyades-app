// load_test.js - Orquestador de la Prueba de Carga Estándar (Load Test)

import { loadProfile } from './config.js';
import { runUserRegistrationVU } from './runner.js';

export const options = loadProfile;
export default runUserRegistrationVU;
