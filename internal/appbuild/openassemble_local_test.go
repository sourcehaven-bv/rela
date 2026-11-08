//go:build !postgres

package appbuild_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

// TestOpenAndAssemble_RefusedWithoutDSNStore pins the fail-closed choice on
// every build but postgres. Their data lives in the project directory, so a
// per-tenant open would hand every tenant the same data.
func TestOpenAndAssemble_RefusedWithoutDSNStore(t *testing.T) {
	base := newSharedBase(t)
	svc, err := base.OpenAndAssemble(context.Background(), "host=tenant-a", audit.Nop{})
	if err == nil {
		_ = svc.Close()
		t.Fatal("OpenAndAssemble must refuse on a build that keeps data in the project directory")
	}
	if svc != nil {
		t.Fatal("OpenAndAssemble returned services alongside its error")
	}
	if !strings.Contains(err.Error(), "postgres build") {
		t.Errorf("error %q should name the build that supports it", err)
	}
}
