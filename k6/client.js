// client.js - Responsabilidad Única: Comunicación HTTP con los endpoints del microservicio

import http from 'k6/http';

const JSON_HEADERS = {
  'Content-Type': 'application/json',
};

/**
 * Envía una petición POST al endpoint de registro de usuarios.
 * Etiqueta la petición para desglose granular en las métricas de k6 y Prometheus.
 *
 * @param {string} baseUrl - URL base del microservicio
 * @param {string} payload - Payload JSON del usuario
 * @returns {object} Objeto de respuesta HTTP provisto por k6
 */
export function postRegisterUser(baseUrl, payload) {
  const endpoint = `${baseUrl}/api/v1/users`;
  const params = {
    headers: JSON_HEADERS,
    tags: { name: 'RegisterUser' },
  };

  return http.post(endpoint, payload, params);
}
