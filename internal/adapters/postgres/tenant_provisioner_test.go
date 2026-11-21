package postgres_test

import (
	"context"
	"strings"
	"testing"

	"pleyades-app/internal/adapters/postgres"
)

// TestLocalPostgresTenantProvisionerSlugValidation audita que los identificadores de slug
// cumplan con las invariantes de longitud y caracteres, previniendo colisiones por truncamiento
// a 63 bytes (NAMEDATALEN) en PostgreSQL.
func TestLocalPostgresTenantProvisionerSlugValidation(t *testing.T) {
	prov := postgres.NewLocalPostgresTenantProvisioner(nil, "localhost", 5432)

	tests := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		{
			name:    "Valid slug passes",
			slug:    "acme-corp",
			wantErr: false,
		},
		{
			name:    "Slug exactly 3 chars passes",
			slug:    "abc",
			wantErr: false,
		},
		{
			name:    "Slug exactly 30 chars passes",
			slug:    "a" + strings.Repeat("b", 28) + "c",
			wantErr: false,
		},
		{
			name:    "Empty slug fails",
			slug:    "",
			wantErr: true,
		},
		{
			name:    "Slug shorter than 3 chars fails",
			slug:    "ab",
			wantErr: true,
		},
		{
			name:    "Slug with 31 chars fails (prevents PostgreSQL truncation)",
			slug:    "a" + strings.Repeat("b", 29) + "c",
			wantErr: true,
		},
		{
			name:    "Slug with 50 chars fails",
			slug:    strings.Repeat("a", 50),
			wantErr: true,
		},
		{
			name:    "Slug with 64 chars fails (would exceed PostgreSQL NAMEDATALEN 63)",
			slug:    strings.Repeat("a", 64),
			wantErr: true,
		},
		{
			name:    "Slug with uppercase fails",
			slug:    "AcmeCorp",
			wantErr: true,
		},
		{
			name:    "Slug starting with hyphen fails",
			slug:    "-acme",
			wantErr: true,
		},
		{
			name:    "Slug ending with hyphen fails",
			slug:    "acme-",
			wantErr: true,
		},
		{
			name:    "Slug with SQL injection characters fails",
			slug:    `acme"; DROP DATABASE "public`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run("Provision_"+tt.name, func(t *testing.T) {
			_, err := prov.Provision(context.Background(), tt.slug)
			if (err != nil) != tt.wantErr {
				t.Errorf("Provision() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})

		t.Run("ResetPassword_"+tt.name, func(t *testing.T) {
			_, err := prov.ResetPassword(context.Background(), tt.slug)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResetPassword() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
