package domain_test

import (
	"errors"
	"strings"
	"testing"

	"pleyades-app/internal/domain"
)

func TestTenantValidationSuite(t *testing.T) {
	t.Run("Tenant Name Validation", testTenantNameValidation)
	t.Run("Tenant Slug Validation", testTenantSlugValidation)
	t.Run("Tenant Owner ID Validation", testTenantOwnerIDValidation)
	t.Run("Master Tenant Validation", testTenantMasterValidation)
}

func testTenantNameValidation(t *testing.T) {
	tests := []struct {
		name        string
		tenantName  string
		expectedErr error
	}{
		{
			name:        "Empty name fails",
			tenantName:  "",
			expectedErr: domain.ErrEmptyTenantName,
		},
		{
			name:        "Whitespace-only name fails",
			tenantName:  "    ",
			expectedErr: domain.ErrEmptyTenantName,
		},
		{
			name:        "Name longer than 50 characters fails",
			tenantName:  strings.Repeat("a", 51),
			expectedErr: domain.ErrLongTenantName,
		},
		{
			name:        "Valid name passes",
			tenantName:  "Acme Corporation",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := domain.Tenant{Name: tt.tenantName}
			err := tenant.ValidateName()
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("ValidateName() error = %v, expectedErr = %v", err, tt.expectedErr)
			}
		})
	}
}

func testTenantSlugValidation(t *testing.T) {
	tests := []struct {
		name        string
		slug        string
		expectedErr error
	}{
		{
			name:        "Empty slug fails",
			slug:        "",
			expectedErr: domain.ErrEmptyTenantSlug,
		},
		{
			name:        "Whitespace-only slug fails",
			slug:        "   ",
			expectedErr: domain.ErrEmptyTenantSlug,
		},
		{
			name:        "Slug shorter than 3 characters fails",
			slug:        "ab",
			expectedErr: domain.ErrShortTenantSlug,
		},
		{
			name:        "Slug longer than 30 characters fails",
			slug:        strings.Repeat("a", 31),
			expectedErr: domain.ErrLongTenantSlug,
		},
		{
			name:        "Slug starting with a number fails",
			slug:        "1acme",
			expectedErr: domain.ErrInvalidTenantSlug,
		},
		{
			name:        "Slug with uppercase characters fails",
			slug:        "AcmeCorp",
			expectedErr: domain.ErrInvalidTenantSlug,
		},
		{
			name:        "Slug with underscores fails",
			slug:        "acme_corp",
			expectedErr: domain.ErrInvalidTenantSlug,
		},
		{
			name:        "Slug starting with a hyphen fails",
			slug:        "-acme",
			expectedErr: domain.ErrInvalidTenantSlug,
		},
		{
			name:        "Slug ending with a hyphen fails",
			slug:        "acme-",
			expectedErr: domain.ErrInvalidTenantSlug,
		},
		{
			name:        "Valid simple slug passes",
			slug:        "acme",
			expectedErr: nil,
		},
		{
			name:        "Valid hyphenated slug passes",
			slug:        "acme-corp",
			expectedErr: nil,
		},
		{
			name:        "Valid alphanumeric slug passes",
			slug:        "acme2026",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := domain.Tenant{Slug: tt.slug}
			err := tenant.ValidateSlug()
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("ValidateSlug() error = %v, expectedErr = %v", err, tt.expectedErr)
			}
		})
	}
}

func testTenantOwnerIDValidation(t *testing.T) {
	tests := []struct {
		name        string
		ownerID     string
		expectedErr error
	}{
		{
			name:        "Empty owner user ID fails",
			ownerID:     "",
			expectedErr: domain.ErrEmptyTenantOwnerID,
		},
		{
			name:        "Whitespace-only owner user ID fails",
			ownerID:     "   ",
			expectedErr: domain.ErrEmptyTenantOwnerID,
		},
		{
			name:        "Valid owner user ID passes",
			ownerID:     "d3b07384-d113-4660-84c6-47b2b8015392",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := domain.Tenant{OwnerUserID: tt.ownerID}
			err := tenant.ValidateOwnerUserID()
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("ValidateOwnerUserID() error = %v, expectedErr = %v", err, tt.expectedErr)
			}
		})
	}
}

func testTenantMasterValidation(t *testing.T) {
	validTenant := domain.Tenant{
		Name:        "Acme Global",
		Slug:        "acme-global",
		OwnerUserID: "d3b07384-d113-4660-84c6-47b2b8015392",
	}

	t.Run("Valid tenant passes master validation", func(t *testing.T) {
		if err := validTenant.Validate(); err != nil {
			t.Errorf("expected nil error on valid tenant, got %v", err)
		}
	})

	t.Run("Invalid name short-circuits master validation", func(t *testing.T) {
		invalid := validTenant
		invalid.Name = ""
		if err := invalid.Validate(); !errors.Is(err, domain.ErrEmptyTenantName) {
			t.Errorf("expected ErrEmptyTenantName, got %v", err)
		}
	})

	t.Run("Invalid slug short-circuits master validation", func(t *testing.T) {
		invalid := validTenant
		invalid.Slug = "INVALID"
		if err := invalid.Validate(); !errors.Is(err, domain.ErrInvalidTenantSlug) {
			t.Errorf("expected ErrInvalidTenantSlug, got %v", err)
		}
	})

	t.Run("Invalid owner ID short-circuits master validation", func(t *testing.T) {
		invalid := validTenant
		invalid.OwnerUserID = ""
		if err := invalid.Validate(); !errors.Is(err, domain.ErrEmptyTenantOwnerID) {
			t.Errorf("expected ErrEmptyTenantOwnerID, got %v", err)
		}
	})
}
