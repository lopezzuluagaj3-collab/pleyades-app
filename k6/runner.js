// runner.js - Responsabilidad Única: Encapsular el ciclo de vida de iteración de un Virtual User (VU)

import { sleep } from 'k6';
import { BASE_URL } from './config.js';
import { generateValidUserPayload } from './generators.js';
import { postRegisterUser } from './client.js';
import { validateRegisterResponse } from './assertions.js';

/**
 * Ejecuta una iteración completa de un usuario virtual:
 * 1. Genera payload único respetando reglas de dominio.
 * 2. Transmite la petición POST HTTP.
 * 3. Valida la respuesta y alimenta las métricas SRE.
 * 4. Aplica una pequeña pausa (think time) de 100ms para simular comportamiento humano.
 */
export function runUserRegistrationVU() {
  const payload = generateValidUserPayload(__VU, __ITER);
  const response = postRegisterUser(BASE_URL, payload);
  validateRegisterResponse(response);
  sleep(0.1);
}
