package cli

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// The regression from BUG-NEQRY2 / BUG-4KPN2M: one entity file with
// unparseable YAML frontmatter used to leave `rela validate` printing
// "All validations passed." and exiting 0, because the store iterator
// error was logged at WARN and the surviving entities were validated as
// if they were the whole project.

// truncatingStore yields the first entity and then a parse failure,
// standing in for the unparseable file that shipped in PR #1314.
//
// armed gates the failure so the appbuildtest fixture can build against
// the same store: construction backfills a search index by scanning
// every entity, and a store that fails there aborts the fixture rather
// than the code under test. Tests arm it once the fixture is up.
type truncatingStore struct {
	store.Store
	err   error
	armed bool
}

func (s *truncatingStore) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	if !s.armed {
		return s.Store.ListEntities(ctx, q)
	}
	return func(yield func(*entity.Entity, error) bool) {
		first := true
		for e, err := range s.Store.ListEntities(ctx, q) {
			if err != nil {
				yield(nil, err)
				return
			}
			if !first {
				yield(nil, s.err)
				return
			}
			first = false
			if !yield(e, nil) {
				return
			}
		}
		yield(nil, s.err)
	}
}

var errFrontmatter = errors.New(
	"entities/bugs/AM-feed-field-redaction.md: failed to parse frontmatter: " +
		"yaml: line 4: mapping values are not allowed in this context")

// validateFixtureMeta declares a rule every seeded entity satisfies, so
// a reported violation can only come from the entity set.
func validateFixtureMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"bug": {Label: "Bug", IDPrefix: "BUG-"},
		},
		Validations: []metamodel.ValidationRule{{
			Name:        "bug-not-bad",
			Description: "Bugs must not be status=bad",
			EntityType:  "bug",
			When:        []string{"status=bad"},
			Then:        []string{"status!=bad"},
			Severity:    "error",
		}},
	}
	meta.InitAliases()
	return meta
}

// runChecksWithStore runs the entity checks against st.
//
// The appbuild fixture is built on the UNDERLYING store: its
// construction backfills an index by scanning entities, which a
// deliberately-failing store would abort. The failing store is injected
// where the bug lives — the analysis service and the properties check
// — which is precisely the read path `rela validate` exercises.
func runChecksWithStore(t *testing.T, meta *metamodel.Metamodel, st *truncatingStore, checks []string) checkOutcome {
	t.Helper()
	svc := appbuildtest.New(meta, appbuildtest.WithStore(st))
	// Arm only now: the fixture's index backfill above must see a
	// healthy store, the checks below must see the broken one. A
	// store with no error configured stays a plain pass-through, which
	// is how the healthy-project guard runs.
	st.armed = st.err != nil
	an, err := analysis.New(analysis.Deps{
		Store:       svc.Store(),
		Meta:        svc.Meta(),
		Tracer:      tracer.New(svc.Store()),
		LuaReadDeps: svc.LuaReadDeps(),
	})
	if err != nil {
		t.Fatalf("analysis.New: %v", err)
	}
	outcome, err := runValidationChecks(context.Background(), svc, an, out, meta, checks)
	if err != nil {
		t.Fatalf("runValidationChecks: %v", err)
	}
	return outcome
}

func seedBugs(meta *metamodel.Metamodel) store.Store {
	st := memstore.New()
	ss := &storeSeeder{s: st, meta: meta}
	ss.addEntity(testutil.EntityFor(meta, "bug").ID("BUG-001"))
	ss.addEntity(testutil.EntityFor(meta, "bug").ID("BUG-002"))
	return st
}

// TestValidate_UnreadableEntityIsNotAPass is the headline regression:
// a truncated scan must not report success, in output or exit code.
func TestValidate_UnreadableEntityIsNotAPass(t *testing.T) {
	meta := validateFixtureMeta(t)
	st := &truncatingStore{Store: seedBugs(meta), err: errFrontmatter}

	buf := withOutput(t, output.FormatTable)
	outcome := runChecksWithStore(t, meta, st, []string{"validations", "properties"})

	if !outcome.incomplete() {
		t.Fatal("outcome.incomplete() = false on a truncated scan: " +
			"validate would exit 0 over input it could not read (BUG-NEQRY2)")
	}
	if !analysis.IsIncompleteScan(outcome.scanErr) {
		t.Errorf("scanErr = %v, want an incomplete-scan error", outcome.scanErr)
	}
	if got := buf.String(); strings.Contains(got, "All validation rules passed") {
		t.Errorf("output claims all rules passed over a partial read:\n%s", got)
	}
}

// TestValidate_IncompleteScanExitCode pins the documented scheme: an
// incomplete run exits 2, distinct from the exit 1 of a run that
// evaluated everything and found violations.
func TestValidate_IncompleteScanExitCode(t *testing.T) {
	meta := validateFixtureMeta(t)
	st := &truncatingStore{Store: seedBugs(meta), err: errFrontmatter}

	withOutput(t, output.FormatTable)
	outcome := runChecksWithStore(t, meta, st, []string{"validations"})
	if !outcome.incomplete() {
		t.Fatal("expected an incomplete outcome")
	}

	// Mirrors the decision in ValidateCmd.Run.
	var exitErr *relaerrors.ExitError
	err := error(relaerrors.NewExitError(exitValidationPartial))
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("exitValidationPartial = %d, want 2", exitValidationPartial)
	}
	if exitValidationFailed != 1 {
		t.Errorf("exitValidationFailed = %d, want 1", exitValidationFailed)
	}
	if exitValidationPartial == exitValidationFailed {
		t.Error("incomplete and violation exits must be distinguishable")
	}
}

// TestValidate_IncompleteScanNamesFile covers acceptance criterion 1.
func TestValidate_IncompleteScanNamesFile(t *testing.T) {
	meta := validateFixtureMeta(t)
	st := &truncatingStore{Store: seedBugs(meta), err: errFrontmatter}

	withOutput(t, output.FormatTable)
	outcome := runChecksWithStore(t, meta, st, []string{"validations"})

	files := analysis.IncompleteScanFiles(outcome.scanErr)
	want := "entities/bugs/AM-feed-field-redaction.md"
	if len(files) == 0 {
		t.Fatalf("no file named in %v", outcome.scanErr)
	}
	found := false
	for _, f := range files {
		if f == want {
			found = true
		}
	}
	if !found {
		t.Errorf("IncompleteScanFiles = %v, want to include %s", files, want)
	}
}

// TestValidate_HealthyProjectStillPasses is the false-positive guard
// (acceptance criterion 3): fail-closed must not become fail-noisy.
func TestValidate_HealthyProjectStillPasses(t *testing.T) {
	meta := validateFixtureMeta(t)

	buf := withOutput(t, output.FormatTable)
	outcome := runChecksWithStore(t, meta, &truncatingStore{Store: seedBugs(meta)}, []string{"validations", "properties"})

	if outcome.incomplete() {
		t.Fatalf("healthy project reported an incomplete scan: %v", outcome.scanErr)
	}
	if outcome.hasErrors {
		t.Error("healthy project reported errors")
	}
	if got := buf.String(); !strings.Contains(got, "All validation rules passed") {
		t.Errorf("healthy project lost its success message:\n%s", got)
	}
}

// TestValidate_ViolationOnEntityAfterUnreadableOne is the sharper pin
// the ticket asks for: the violation sits on an entity that follows the
// unreadable one in scan order, so it is exactly what the old code
// dropped. The run must not be green.
func TestValidate_ViolationOnEntityAfterUnreadableOne(t *testing.T) {
	meta := validateFixtureMeta(t)
	base := memstore.New()
	ss := &storeSeeder{s: base, meta: meta}
	ss.addEntity(testutil.EntityFor(meta, "bug").ID("BUG-001"))
	// Never reached by the scan: it is behind the parse failure.
	ss.addEntity(testutil.EntityFor(meta, "bug").ID("BUG-002").With("status", "bad"))
	st := &truncatingStore{Store: base, err: errFrontmatter}

	buf := withOutput(t, output.FormatTable)
	outcome := runChecksWithStore(t, meta, st, []string{"validations"})

	if !outcome.incomplete() {
		t.Fatal("a violation hidden behind an unreadable entity produced a complete-looking run")
	}
	if got := buf.String(); strings.Contains(got, "All validation rules passed") {
		t.Errorf("output claims a pass while a violation was never evaluated:\n%s", got)
	}
}
