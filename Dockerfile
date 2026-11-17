# ==============================================================================
# Multi-Stage Dockerfile para Pleyades App (Cloud-Native Microservice)
# Arquitectura: Multi-tenant SaaS Engine para Kubernetes (AKS)
#
# Etapa 1: Builder (Compilación estática y dependencias)
# Etapa 2: Runtime (Distroless estático y seguro sin privilegios de root)
# ==============================================================================

# ------------------------------------------------------------------------------
# Etapa 1: Compilación (Builder)
# ------------------------------------------------------------------------------
FROM golang:alpine AS builder

# Instalar dependencias del sistema necesarias para certificados y zonas horarias
RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /build

# 1. Cache de dependencias de Go (Aprovechar caché de capas de Docker)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# 2. Copiar código fuente
COPY . .

# 3. Compilación estática optimizada para producción:
# - CGO_ENABLED=0: Binario 100% estático sin dependencias de libc del host.
# - GOOS=linux GOARCH=amd64: Arquitectura estándar para clústeres Kubernetes en Azure (AKS).
# - -ldflags="-s -w -extldflags '-static'":
#     -s: Omite la tabla de símbolos (reduce tamaño drásticamente).
#     -w: Omite la información de depuración DWARF.
# - -trimpath: Remueve rutas absolutas del host en stacktraces para mayor seguridad e higiene.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o /build/bin/pleyades \
    ./cmd/pleyades-app

# ------------------------------------------------------------------------------
# Etapa 2: Imagen Final de Producción (Distroless Runtime)
# ------------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

LABEL maintainer="Equipo de Arquitectura Pleyades <arquitecto@pleyades.io>"
LABEL description="Pleyades Multi-Tenant Microservice API & SaaS Provisioner"
LABEL org.opencontainers.image.source="https://github.com/lopezzuluagaj3-collab/pleyades-app"

WORKDIR /app

# Copiar certificados raíz TLS y zonas horarias desde el builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copiar el binario estático compilado
COPY --from=builder /build/bin/pleyades /app/pleyades

# Puerto de escucha del microservicio
EXPOSE 8080

# Seguridad: Ejecutar bajo el usuario sin privilegios 'nonroot' (UID 65532)
# Cumple con el estándar Restricted de Pod Security Standards de Kubernetes
USER nonroot:nonroot

# Punto de entrada de ejecución
ENTRYPOINT ["/app/pleyades"]
