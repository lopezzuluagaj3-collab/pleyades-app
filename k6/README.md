# Arnés de Pruebas de Carga y Resiliencia con k6 (`k6/`)

Este arnés implementa una suite completa de pruebas de rendimiento para evaluar la estabilidad, latencia en percentiles críticos (p95, p99), comportamiento ante la saturación de CPU (hashing Bcrypt), concurrencia sobre el pool de PostgreSQL y autoescalado con **Kubernetes HPA (Horizontal Pod Autoscaler)**.

---

## 🏛️ Arquitectura Modular (SOLID / OCP / SRP)

Cada componente posee una responsabilidad única y desacoplada, permitiendo reutilizar el generador, el cliente HTTP y las aserciones en múltiples escenarios sin duplicar código:

```
k6/
├── config.js       # SRP: Parámetros de entorno (TARGET_URL), perfiles de etapas y SLOs (Thresholds).
├── generators.js   # SRP: Generación de datos sintéticos con unicidad garantizada respetando las reglas de dominio.
├── client.js       # SRP: Transporte HTTP y etiquetado para métricas de Prometheus.
├── assertions.js   # SRP: Aserciones atómicas de respuesta y métricas de Golden Signals del cliente.
├── runner.js       # SRP: Encapsula el ciclo de iteración del Virtual User (VU) con think-time.
│
│── Escenarios Ejecutables:
├── load_test.js    # 1. Prueba de Carga Estándar (Tráfico previsto y validación de SLOs de negocio).
├── stress_test.js  # 2. Prueba de Estrés (Punto de quiebre y activación de réplicas HPA en K8s).
├── soak_test.js    # 3. Prueba de Resistencia / Soak (Detección de memory leaks y fugas de conexiones en K8s).
├── spike_test.js   # 4. Prueba de Picos (Tolerancia a avalanchas instantáneas en el Ingress Controller).
└── README.md       # Guía de ejecución y análisis.
```

---

## 📋 Comparativa de Escenarios

| Escenario | Script | Duración | Carga Máxima | Objetivo Principal |
| :--- | :--- | :--- | :--- | :--- |
| **Carga (Load)** | `load_test.js` | ~1.5 min | 50 VUs | Validar latencias p95 < 250ms bajo tráfico nominal. |
| **Estrés (Stress)** | `stress_test.js` | ~6 min | 300 VUs | Encontrar el punto de quiebre y validar autoescalado del pod. |
| **Resistencia (Soak)** | `soak_test.js` | 30m - 2h (Configurable) | 20 VUs (Constante) | Detectar fugas de memoria (RAM), acumulación de conexiones muertas en DB y saturación de GC. |
| **Picos (Spike)** | `spike_test.js` | ~1 min | 200 VUs (En 10s) | Evaluar si el Ingress rechaza peticiones o satura buffers. |

---

## 🚀 Guía de Ejecución

### 1. Prueba de Carga Estándar (Local o K8s)
```powershell
cd D:\proyectos\pleyades\pleyades-app
k6 run k6/load_test.js
```

---

### 2. Prueba de Estrés (Validación de HPA en Kubernetes)
Diseñada para observar cómo Kubernetes reacciona ante la saturación de CPU provocada por el hashing Bcrypt (`cost=10`):

```powershell
# Ejecutar contra el Ingress o IP pública de AKS
k6 run -e TARGET_URL="http://<INGRESS_O_IP_AKS>:8080" k6/stress_test.js
```

#### Monitoreo simultáneo en Kubernetes:
En otra terminal, observa el autoescalado en tiempo real:
```powershell
kubectl get hpa -w
kubectl top pods
```
*Deberías ver cómo el HPA incrementa las réplicas (ej: de 1 pod a 3 o 4 pods) conforme la carga supera el 70% de CPU objetivo.*

---

### 3. Prueba de Resistencia y Memoria (Soak / Endurance Test)
Esta prueba está concebida para ejecutarse **fuera del proyecto contra el clúster en Kubernetes (AKS)** para auditar el comportamiento a largo plazo:

```powershell
# Ejecución por defecto (30 minutos a 20 usuarios constantes)
k6 run -e TARGET_URL="http://<INGRESS_O_IP_AKS>:8080" k6/soak_test.js

# O personalizar duración (ej: 1 hora) y concurrencia (ej: 30 VUs)
k6 run -e TARGET_URL="http://<INGRESS_O_IP_AKS>:8080" -e SOAK_DURATION="1h" -e SOAK_VUS="30" k6/soak_test.js
```

#### Qué auditar durante la prueba de Resistencia:
1. **Fugas de Memoria (Memory Leaks):**
   - En Grafana o `kubectl top pod`, la memoria RAM no debe crecer infinitamente de forma lineal. Debe subir y estabilizarse en una meseta tras los ciclos de recolección de basura (*Garbage Collector* de Go).
2. **Pool de PostgreSQL:**
   - Verifica en PostgreSQL que las conexiones inactivas no queden huérfanas:
     ```sql
     SELECT state, count(*) FROM pg_stat_activity GROUP BY state;
     ```
3. **Degradación de Latencia:**
   - La latencia p95 al minuto 50 debe ser similar a la del minuto 5 (<300ms). Si sube progresivamente, indica retención indebida de recursos en memoria.

---

### 4. Prueba de Picos Repentinos (Spike Test)
```powershell
k6 run -e TARGET_URL="http://<INGRESS_O_IP_AKS>:8080" k6/spike_test.js
```

---

## 📊 Correlación con Métricas de Prometheus

Durante cualquier prueba, consulta las métricas del microservicio:
```powershell
Invoke-RestMethod -Uri http://localhost:8080/metrics
```
Métricas clave de saturación:
- `http_requests_in_flight`: Peticiones activas procesándose en simultáneo.
- `http_requests_total`: Conteo total acumulado por código de estado (`201`, `400`, `409`, `500`).
- `http_request_duration_seconds_bucket`: Distribución de latencias para cálculo de percentiles p50, p95 y p99.
