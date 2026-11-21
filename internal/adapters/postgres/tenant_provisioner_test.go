package postgres_test

import (
	"context"
	"strings"
	"testing"

	"pleyades-app/internal/adapters/postgres"
)

// TestValidateSlug audita que los identificadores de slug cumplan con las invariantes de
// longitud y caracteres, previniendo colisiones por truncamiento a 63 bytes (NAMEDATALEN) en PostgreSQL.
func TestValidateSlug(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			err := postgres.ValidateSlug(tt.slug)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSlug(%q) error = %v, wantErr = %v", tt.slug, err, tt.wantErr)
			}
		})
	}
}

// TestLocalPostgresTenantProvisioner_RejectsInvalidSlugBeforeDB verifies that invalid slugs
// fail fast at the validation boundary without attempting database access.
func TestLocalPostgresTenantProvisioner_RejectsInvalidSlugBeforeDB(t *testing.T) {
	prov := postgres.NewLocalPostgresTenantProvisioner(nil, "localhost", 5432)

	invalidSlugs := []string{
		"",
		"a",
		"ab",
		"AcmeCorp",
		"-leading-hyphen",
		"trailing-hyphen-",
		strings.Repeat("a", 31),
		strings.Repeat("a", 64),
		`injection'; DROP TABLE tenants;--`,
	}

	for _, slug := range invalidSlugs {
		t.Run("Provision_"+slug, func(t *testing.T) {
			_, err := prov.Provision(context.Background(), slug)
			if err == nil {
				t.Errorf("expected error for slug %q, got nil", slug)
			}
		})

		t.Run("ResetPassword_"+slug, func(t *testing.T) {
			_, err := prov.ResetPassword(context.Background(), slug)
			if err == nil {
				t.Errorf("expected error for slug %q, got nil", slug)
			}
		})
	}
}

// TestTenantIsolationAndConnectionLimitDDL audita de forma automatizada en CI la construcción
// de las sentencias DDL para aislamiento estricto (REVOKE CONNECT) y cuotas de conexión (CONNECTION LIMIT 2).
func TestTenantIsolationAndConnectionLimitDDL(t *testing.T) {
	// 1. Verificar construcción segura de REVOKE CONNECT
	revokeDDL := postgres.BuildRevokePublicConnectDDL("tenant_acmecloud")
	expectedRevoke := `REVOKE CONNECT ON DATABASE "tenant_acmecloud" FROM PUBLIC`
	if revokeDDL != expectedRevoke {
		t.Errorf("BuildRevokePublicConnectDDL() = %q, want %q", revokeDDL, expectedRevoke)
	}

	// 2. Verificar construcción de CREATE USER con CONNECTION LIMIT
	createUserDDL := postgres.BuildCreateUserDDL("usr_acmecloud", "Secret'Pass#123", postgres.DefaultTenantConnectionLimit)
	expectedCreate := `CREATE USER "usr_acmecloud" WITH ENCRYPTED PASSWORD 'Secret''Pass#123' CONNECTION LIMIT 2`
	if createUserDDL != expectedCreate {
		t.Errorf("BuildCreateUserDDL() = %q, want %q", createUserDDL, expectedCreate)
	}

	// 3. Verificar construcción de ALTER ROLE con CONNECTION LIMIT
	alterRoleDDL := postgres.BuildAlterUserConnectionLimitDDL("usr_acmecloud", postgres.DefaultTenantConnectionLimit)
	expectedAlter := `ALTER ROLE "usr_acmecloud" CONNECTION LIMIT 2`
	if alterRoleDDL != expectedAlter {
		t.Errorf("BuildAlterUserConnectionLimitDDL() = %q, want %q", alterRoleDDL, expectedAlter)
	}

	// 4. Verificar construcción de GRANT ALL PRIVILEGES
	grantDDL := postgres.BuildGrantPrivilegesDDL("tenant_acmecloud", "usr_acmecloud")
	expectedGrant := `GRANT ALL PRIVILEGES ON DATABASE "tenant_acmecloud" TO "usr_acmecloud"`
	if grantDDL != expectedGrant {
		t.Errorf("BuildGrantPrivilegesDDL() = %q, want %q", grantDDL, expectedGrant)
	}
}
