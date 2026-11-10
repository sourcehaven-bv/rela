//go:build !postgres

package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TestTenantCommand_NeedsPostgres pins that a build without PostgreSQL
// refuses every lifecycle command.
func TestTenantCommand_NeedsPostgres(t *testing.T) {
	tio := productionTenantIO()
	var errOut bytes.Buffer
	tio.in, tio.out, tio.err = strings.NewReader(""), &bytes.Buffer{}, &errOut
	tio.getenv = func(string) string { return "postgres://db.example/rela" }
	code := runTenant(t.Context(), []string{"create", "-project", projectDir(t), "org-a"}, tio)
	if code != 1 || !strings.Contains(errOut.String(), "postgres build") {
		t.Errorf("exit %d, stderr %q; want a postgres-build refusal", code, errOut.String())
	}
	if _, err := tenant.NewProvisioner("x", "y"); !errors.Is(err, tenant.ErrUnsupportedBuild) {
		t.Errorf("NewProvisioner = %v, want ErrUnsupportedBuild", err)
	}
}
