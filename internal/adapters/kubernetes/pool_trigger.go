package kubernetes

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"pleyades-app/internal/ports"
)

var _ ports.PoolScaleTrigger = (*K8sPoolScaleTrigger)(nil)
var _ ports.PoolScaleTrigger = (*NoOpPoolScaleTrigger)(nil)

// K8sPoolScaleTrigger emite órdenes declarativas de FlexibleServer a Crossplane mediante el API REST de Kubernetes
type K8sPoolScaleTrigger struct {
	client    *http.Client
	k8sHost   string
	tokenPath string
	enabled   bool
}

// NewK8sPoolScaleTrigger inicializa el cliente REST seguro contra el control plane de Kubernetes
func NewK8sPoolScaleTrigger(enabled bool) *K8sPoolScaleTrigger {
	t := &K8sPoolScaleTrigger{
		k8sHost:   "https://kubernetes.default.svc",
		tokenPath: "/var/run/secrets/kubernetes.io/serviceaccount/token", // #nosec G101 - Safe static path to projected service account token
		enabled:   enabled,
	}

	caCertPath := "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	caCert, err := os.ReadFile(caCertPath) // #nosec G304 - Fixed trusted path for in-cluster CA
	if err == nil {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		t.client = &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:    caCertPool,
					MinVersion: tls.VersionTLS12,
				},
			},
		}
	} else {
		t.client = &http.Client{Timeout: 5 * time.Second}
	}

	return t
}

// TriggerWarmPool emite el manifiesto declarativo de FlexibleServer para que Crossplane lo reconcilie en Azure
func (t *K8sPoolScaleTrigger) TriggerWarmPool(ctx context.Context, serverName string) error {
	if !t.enabled {
		return nil
	}

	tokenBytes, err := os.ReadFile(t.tokenPath) // #nosec G304 - Trusted in-cluster service account path
	if err != nil {
		return fmt.Errorf("k8s trigger: no se pudo leer el token del ServiceAccount: %w", err)
	}
	token := string(tokenBytes)

	payload := map[string]interface{}{
		"apiVersion": "dbforpostgresql.azure.upbound.io/v1beta2",
		"kind":       "FlexibleServer",
		"metadata": map[string]interface{}{
			"name": serverName,
			"labels": map[string]string{
				"app.kubernetes.io/managed-by": "crossplane",
				"pleyades.io/pool-tier":        "standard",
				"pleyades.io/pool-capacity":    "7",
			},
		},
		"spec": map[string]interface{}{
			"providerConfigRef": map[string]string{
				"name": "default",
			},
			"writeConnectionSecretToRef": map[string]string{
				"name":      fmt.Sprintf("%s-conn", serverName),
				"namespace": "crossplane-system",
			},
			"forProvider": map[string]interface{}{
				"location":                   "centralus",
				"version":                    "16",
				"skuName":                    "B_Standard_B1ms",
				"storageMb":                  32768,
				"autoGrowEnabled":            true,
				"publicNetworkAccessEnabled": true,
				"resourceGroupName":          "rg-pleyades-dev",
				"administratorLogin":         "pleyades_admin",
				"autoGeneratePassword":       true,
				"administratorPasswordSecretRef": map[string]string{
					"name":      fmt.Sprintf("%s-admin-secret", serverName),
					"namespace": "crossplane-system",
					"key":       "password",
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("k8s trigger: fallo al serializar payload: %w", err)
	}

	url := fmt.Sprintf("%s/apis/dbforpostgresql.azure.upbound.io/v1beta2/flexibleservers", t.k8sHost)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("k8s trigger: fallo al crear request HTTP: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("k8s trigger: error de comunicación con Kubernetes API: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}

	return fmt.Errorf("k8s trigger: Kubernetes API retornó código HTTP inesperado: %d", resp.StatusCode)
}

// NoOpPoolScaleTrigger implementa el disparador nulo para pruebas y entornos locales
type NoOpPoolScaleTrigger struct{}

// NewNoOpPoolScaleTrigger crea un disparador no operacional
func NewNoOpPoolScaleTrigger() *NoOpPoolScaleTrigger {
	return &NoOpPoolScaleTrigger{}
}

// TriggerWarmPool simula la aceptación del aprovisionamiento sin invocar K8s
func (n *NoOpPoolScaleTrigger) TriggerWarmPool(ctx context.Context, serverName string) error {
	// No-op: utilizado para pruebas unitarias y desarrollo local sin clúster Kubernetes
	return nil
}
