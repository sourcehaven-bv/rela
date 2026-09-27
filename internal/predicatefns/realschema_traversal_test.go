package predicatefns

import (
	"os"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// The motivating case from RES-RELTRV, against the REAL tickets schema:
// `caused-by` points at four types whose `status` enums differ, so the bare
// form must be refused and the ascribed form accepted.
func TestValidateTraversals_AgainstRealSchema(t *testing.T) {
	raw, err := os.ReadFile("../../tickets/schema.yaml")
	if err != nil {
		t.Skipf("schema not readable: %v", err)
	}
	// Parse, not a bare yaml.Unmarshal: the inverse-ID index is populated by
	// the loader, and the incoming case below depends on it.
	meta, err := metamodel.Parse(raw)
	if err != nil {
		t.Fatalf("parse tickets schema: %v", err)
	}
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{"status": predicate.StringType}); err != nil {
		t.Fatal(err)
	}
	mustCompile := func(src string) *predicate.Program {
		p, cErr := predicate.Compile(env, src)
		if cErr != nil {
			t.Fatalf("compile %q: %v", src, cErr)
		}
		return p
	}

	bare := ValidateTraversals(meta, "bug", mustCompile(
		`related(entity, 'caused-by', { status = 'done' })`))
	if bare == nil || !strings.Contains(bare.Error(), "add type=") {
		t.Fatalf("bare union traversal must be refused, got %v", bare)
	}
	t.Logf("refusal: %v", bare)

	if err := ValidateTraversals(meta, "bug", mustCompile(
		`related(entity, 'caused-by', { type = 'ticket', status = 'done' })`)); err != nil {
		t.Fatalf("ascribed traversal must be accepted: %v", err)
	}

	// TKT-CXQEV0: a view on features filtering on the tickets that implement
	// them walks `implements` backwards through its inverse ID.
	if err := ValidateTraversals(meta, "feature", mustCompile(
		`related(entity, 'implementedBy', { status = 'in-progress' })`)); err != nil {
		t.Fatalf("incoming traversal must be accepted: %v", err)
	}
}
