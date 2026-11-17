# Módulo de Pruebas de Carga con k6 (`k6/`)

Este módulo implementa pruebas de carga progresiva (*Ramping VUs*) para evaluar el rendimiento, la latencia en percentiles críticos (p95, p99) y la estabilidad del microservicio `pleyades-app` bajo estrés, correlacionando los resultados con las métricas expuestas en `/metrics` para Prometheus.

---

## 🏛️ Arquitectura Modular (SOLID / SRP)

El script no es un archivo monolítico; se encuentra desacoplado en componentes con una única responsabilidad:

```
k6/
├── config.js       # Responsabilidad: Variables de entorno, etapas de carga (stages) y SLOs (thresholds).
├── generators.js   # Responsabilidad: Generación de datos sintéticos válidos cumpliendo las invariantes de dominio (emails únicos, formato seguro de contraseñas).
├── client.js       # Responsabilidad: Transporte HTTP y headers hacia el microservicio.
├── assertions.js   # Responsabilidad: Validaciones de respuesta (status 201, headers, UUID) y métricas de Golden Signals.
├── load_test.js    # Responsabilidad: Orquestador puro del ciclo de vida de cada Virtual User (VU).
└── README.md       # Guía de ejecución y análisis de resultados.
```

---

## 📦 Instalación de k6

Si no tienes `k6` instalado en tu máquina:

### En Windows (PowerShell):
```powershell
# Con WinGet (Recomendado)
winget install k6 --source winget

# O con Chocolatey
choco install k6
```

### En Linux (Ubuntu / Debian):
```bash
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6
```

---

## 🚀 Ejecución de las Pruebas

> [!IMPORTANT]
> Asegúrate de que el microservicio y la base de datos PostgreSQL se encuentren ejecutándose antes de iniciar las pruebas:
> ```powershell
> docker start pleyades-postgres
> cd D:\proyectos\pleyades\pleyades-app
> go run ./cmd/pleyades-app
> ```

### 1. Ejecución Completa con Perfil de Carga (Ramping VUs)
Ejecuta el perfil definido en `config.js` (calentamiento de 10 VUs, pico sostenido de 50 VUs y enfriamiento):
```powershell
cd D:\proyectos\pleyades\pleyades-app
k6 run k6/load_test.js
```

### 2. Ejecución Rápida de Humo (Smoke Test - 10 segundos)
Si deseas una verificación rápida sin esperar todo el ciclo:
```powershell
k6 run --vus 5 --duration 10s k6/load_test.js
```

### 3. Ejecución contra un Entorno Remoto o Kubernetes (AKS)
Puedes parametrizar la variable de entorno `TARGET_URL`:
```powershell
k6 run -e TARGET_URL="http://20.120.x.x:8080" k6/load_test.js
```

---

## 📊 Correlación con Métricas de Prometheus

Durante o inmediatamente después de la prueba de carga, puedes inspeccionar las métricas del microservicio en otra terminal:

```powershell
Invoke-RestMethod -Uri http://localhost:8080/metrics
```

Podrás observar:
- `http_requests_total{method="POST",path="/api/v1/users",status="201"}`: Contador exacto de usuarios creados.
- `http_request_duration_seconds_bucket`: Distribución de la latencia en buckets histograma.
- `http_requests_in_flight`: Saturación instantánea de peticiones activas.

---

## 🎯 Criterios de Éxito (SLOs / Thresholds)

La prueba fallará automáticamente en consola si el microservicio no cumple con los siguientes acuerdos de nivel de servicio:
- **`http_req_duration (p95)` < 250ms:** El 95% de las peticiones debe completarse en menos de un cuarto de segundo.
- **`http_req_failed` < 1%:** Menos del 1% de las peticiones pueden fallar (errores 5xx o timeouts).
