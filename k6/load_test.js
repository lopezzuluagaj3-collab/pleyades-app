// load_test.js - Responsabilidad Única: Orquestador principal del escenario de prueba de carga

import { sleep } from 'k6';
import { BASE_URL, loadProfile } from './config.js';
import { generateValidUserPayload } from './generators.js';
import { postRegisterUser } from './client.js';
import { validateRegisterResponse } from './assertions.js';

// Exportación de configuración y SLOs requerida por k6
export const options = loadProfile;

/**
 * Función orquestadora ejecutada por cada Virtual User (VU) en cada iteración.
 * Coordina las responsabilidades atómicas:
 * 1. Generación de datos sintéticos con unicidad garantizada.
 * 2. Transmisión HTTP hacia el microservicio.
 * 3. Verificación de invariantes de respuesta y registro de métricas.
 * 4. Pausa de simulación de comportamiento de usuario real (Think Time).
 */
export default function () {
  // 1. Generar payload de usuario único respetando las invariantes de dominio
  const payload = generateValidUserPayload(__VU, __ITER);

  // 2. Ejecutar petición HTTP POST
  const response = postRegisterUser(BASE_URL, payload);

  // 3. Validar respuesta y registrar métricas
  validateRegisterResponse(response);

  // 4. Think-time realista para evitar flooding artificial no controlado (100ms)
  sleep(0.1);
}
