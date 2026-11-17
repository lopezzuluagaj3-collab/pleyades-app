// assertions.js - Responsabilidad Única: Aserciones atómicas de respuesta y registro de métricas SRE

import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Métricas personalizadas de Golden Signals a nivel de cliente
export const registrationSuccessRate = new Rate('registration_success_rate');
export const registrationsTotal = new Counter('registrations_total');
export const registrationLatency = new Trend('registration_duration_ms');

/**
 * Valida de forma atómica que la respuesta cumpla con los estándares del API:
 * 1. Código HTTP 201 Created.
 * 2. Header Content-Type application/json.
 * 3. Body parseable con campo 'uuid' asignado.
 *
 * Registra los valores en las métricas personalizadas de k6.
 *
 * @param {object} response - Objeto de respuesta HTTP devuelto por k6
 * @returns {boolean} true si todas las comprobaciones fueron satisfactorias
 */
export function validateRegisterResponse(response) {
  const passed = check(response, {
    'status is 201 Created': (r) => r.status === 201,
    'content-type is application/json': (r) =>
      r.headers['Content-Type'] && r.headers['Content-Type'].includes('application/json'),
    'response body contains generated uuid': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body && typeof body.uuid === 'string' && body.uuid.length > 0;
      } catch (err) {
        return false;
      }
    },
  });

  // Alimentar métricas SRE
  registrationSuccessRate.add(passed);
  registrationsTotal.add(1);
  registrationLatency.add(response.timings.duration);

  return passed;
}
