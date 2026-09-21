package predicatefns

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

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
	var meta metamodel.Metamodel
	if err := yaml.Unmarshal(raw, &meta); err != nil {
		t.Skipf("schema not parseable: %v", err)
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

	bare := ValidateTraversals(&meta, "bug", mustCompile(
		`related(entity, 'caused-by', { status = 'done' })`))
	if bare == nil || !strings.Contains(bare.Error(), "add type=") {
		t.Fatalf("bare union traversal must be refused, got %v", bare)
	}
	t.Logf("refusal: %v", bare)

	if err := ValidateTraversals(&meta, "bug", mustCompile(
		`related(entity, 'caused-by', { type = 'ticket', status = 'done' })`)); err != nil {
		t.Fatalf("ascribed traversal must be accepted: %v", err)
	}
}
