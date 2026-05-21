package predicate_test

import (
	"go/build"
	"strings"
	"testing"
)

// TestPackageImports pins the package boundary: predicate must not
// import any rela domain package. New imports added to the engine
// have to be cleared against this list or the test fails.
//
// Acceptance criterion: AC8 (RR-T4CW (d) — internal/lua specifically).
func TestPackageImports(t *testing.T) {
	const pkgPath = "github.com/Sourcehaven-BV/rela/internal/predicate"
	pkg, err := build.Default.Import(pkgPath, ".", 0)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	forbidden := []string{
		"github.com/Sourcehaven-BV/rela/internal/acl",
		"github.com/Sourcehaven-BV/rela/internal/dataentry",
		"github.com/Sourcehaven-BV/rela/internal/entitymanager",
		"github.com/Sourcehaven-BV/rela/internal/store",
		"github.com/Sourcehaven-BV/rela/internal/entity",
		"github.com/Sourcehaven-BV/rela/internal/metamodel",
		"github.com/Sourcehaven-BV/rela/internal/lua",
		"github.com/Sourcehaven-BV/rela/internal/search",
		"github.com/Sourcehaven-BV/rela/internal/tracer",
	}
	for _, imp := range pkg.Imports {
		for _, bad := range forbidden {
			if imp == bad || strings.HasPrefix(imp, bad+"/") {
				t.Errorf("predicate imports forbidden package %q", imp)
			}
		}
	}
}
