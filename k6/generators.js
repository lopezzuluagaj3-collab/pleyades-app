// generators.js - Responsabilidad Única: Generación de datos sintéticos válidos cumpliendo invariantes de dominio

/**
 * Genera un payload válido para el registro de usuarios.
 * Garantiza unicidad mediante el Virtual User ID (__VU), el número de iteración (__ITER) y timestamp.
 * Cumple estrictamente con las reglas de negocio del dominio:
 * - Email con formato válido y único.
 * - Nombre alfabético sin números ni caracteres especiales (<50 caracteres).
 * - Contraseña con mayúsculas, minúsculas, números y caracteres especiales.
 *
 * @param {number} vu - Identificador del usuario virtual actual
 * @param {number} iter - Número de iteración del usuario virtual
 * @returns {string} JSON stringificado listo para envío HTTP
 */
export function generateValidUserPayload(vu, iter) {
  const timestamp = Date.now();
  const randomSuffix = Math.floor(Math.random() * 10000);
  const email = `load_vu${vu}_it${iter}_${timestamp}_${randomSuffix}@pleyades.io`;

  const user = {
    email: email,
    name: 'PerformanceUser',
    password: 'PleyadesPassword123!',
  };

  return JSON.stringify(user);
}
