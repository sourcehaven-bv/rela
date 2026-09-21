package analysis_test

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// The regression these tests pin (BUG-NEQRY2 / BUG-4KPN2M): a store
// iterator that fails part-way used to be swallowed into a partial
// slice, so an analysis that never read an entity reported the same
// result as one that read it and found nothing wrong.

// errAfterNStore wraps a Store and makes ListEntities yield the first n
// entities and then an error, reproducing an unparseable file sitting
// in the middle of a scan.
type errAfterNStore struct {
	store.Store
	n   int
	err error
}

func (s *errAfterNStore) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		i := 0
		for e, err := range s.Store.ListEntities(ctx, q) {
			if err != nil {
				yield(nil, err)
				return
			}
			if i == s.n {
				yield(nil, s.err)
				return
			}
			if !yield(e, nil) {
				return
			}
			i++
		}
		if i <= s.n {
			yield(nil, s.err)
		}
	}
}

// errParse is the shape the fs store produces for an unparseable file:
// the file key, then the yaml failure. IncompleteScanFiles recovers the
// key from it.
var errParse = errors.New(
	"entities/bugs/AM-feed-field-redaction.md: failed to parse frontmatter: " +
		"yaml: line 4: mapping values are not allowed in this context")

func newFailingService(t *testing.T, meta *metamodel.Metamodel, failAfter int, seed func(store.Store)) *analysis.Service {
	t.Helper()
	base := memstore.New()
	if seed != nil {
		seed(base)
	}
	st := &errAfterNStore{Store: base, n: failAfter, err: errParse}
	tr := tracer.New(st)
	svc, err := analysis.New(analysis.Deps{
		Store:  st,
		Meta:   meta,
		Tracer: tr,
		LuaReadDeps: lua.ReadDeps{
			VisibleReader: st,
			Tracer:        tr,
			Meta:          meta,
		},
	})
	if err != nil {
		t.Fatalf("analysis.New: %v", err)
	}
	return svc
}

func validationMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"bug": {Label: "Bug", IDPrefix: "BUG-"},
		},
		Validations: []metamodel.ValidationRule{{
			Name:        "bug-needs-title",
			Description: "Bugs must have a title",
			EntityType:  "bug",
			Severity:    "error",
			// Every seeded bug satisfies this: none has status=bad, so the
			// rule selects nothing and any violation reported would come
			// from the entity set, not from the rule itself.
			When: []string{"status=bad"},
			Then: []string{"status!=bad"},
		}},
	}
	meta.InitAliases()
	return meta
}

// TestRunValidations_IncompleteScanReportsError is the pin for the
// reported failure: the rules run clean over what they could read, and
// the caller must still learn the scan was partial.
func TestRunValidations_IncompleteScanReportsError(t *testing.T) {
	meta := validationMeta(t)
	svc := newFailingService(t, meta, 1, func(s store.Store) {
		addEntity(s, "BUG-001", "bug", map[string]any{"title": "first"})
		addEntity(s, "BUG-002", "bug", map[string]any{"title": "second"})
	})

	result, err := svc.RunValidations(context.Background(), analysis.Options{})
	if len(result.Violations) != 0 {
		t.Fatalf("got %d violations, want 0 — the fixture is clean on the entities that parse", len(result.Violations))
	}
	if err == nil {
		t.Fatal("RunValidations returned nil error on a truncated scan: " +
			"a clean result over partial input is indistinguishable from a real pass (BUG-4KPN2M)")
	}
	if !analysis.IsIncompleteScan(err) {
		t.Errorf("IsIncompleteScan(%v) = false, want true", err)
	}
	if !errors.Is(err, errParse) {
		t.Errorf("error does not wrap the underlying read failure: %v", err)
	}
}

// TestIncompleteScanFiles_NamesOffendingFile covers acceptance criterion
// 1: the operator must be told which file to fix.
func TestIncompleteScanFiles_NamesOffendingFile(t *testing.T) {
	meta := validationMeta(t)
	svc := newFailingService(t, meta, 0, func(s store.Store) {
		addEntity(s, "BUG-001", "bug", map[string]any{"title": "first"})
	})

	_, err := svc.RunValidations(context.Background(), analysis.Options{})
	files := analysis.IncompleteScanFiles(err)
	want := "entities/bugs/AM-feed-field-redaction.md"
	if len(files) != 1 || files[0] != want {
		t.Fatalf("IncompleteScanFiles = %v, want [%s]", files, want)
	}
}

// TestAnalyses_PropagateIncompleteScan is the per-call-site coverage the
// ticket asks for: every analysis that scans entities must report the
// truncation rather than returning a short slice.
func TestAnalyses_PropagateIncompleteScan(t *testing.T) {
	meta := validationMeta(t)
	seed := func(s store.Store) {
		addEntity(s, "BUG-001", "bug", map[string]any{"title": "first"})
		addEntity(s, "BUG-002", "bug", map[string]any{"title": "second"})
	}

	tests := []struct {
		name string
		call func(*analysis.Service) error
	}{
		{"FindDuplicates", func(s *analysis.Service) error {
			_, err := s.FindDuplicates(context.Background(), analysis.Options{})
			return err
		}},
		{"FindGaps", func(s *analysis.Service) error {
			_, err := s.FindGaps(context.Background(), analysis.Options{})
			return err
		}},
		{"RunValidations", func(s *analysis.Service) error {
			_, err := s.RunValidations(context.Background(), analysis.Options{})
			return err
		}},
		{"RunValidationsFiltered", func(s *analysis.Service) error {
			_, err := s.RunValidationsFiltered(context.Background(), analysis.Options{},
				[]analysis.ValidationFilter{{RuleName: "bug-needs-title"}})
			return err
		}},
		{"AnalyzeAll", func(s *analysis.Service) error {
			_, err := s.AnalyzeAll(context.Background(), analysis.Options{})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFailingService(t, meta, 1, seed)
			err := tt.call(svc)
			if err == nil {
				t.Fatalf("%s returned nil error on a truncated scan, want an incomplete-scan error", tt.name)
			}
			if !analysis.IsIncompleteScan(err) {
				t.Errorf("IsIncompleteScan(%v) = false, want true", err)
			}
		})
	}
}

// TestFindUniqueViolations_PropagatesIncompleteScan is separate because
// it needs a metamodel declaring a unique property; without one the
// method returns before scanning at all.
func TestFindUniqueViolations_PropagatesIncompleteScan(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"bug": {
				Label:    "Bug",
				IDPrefix: "BUG-",
				Properties: map[string]metamodel.PropertyDef{
					"slug": {Type: "string", Unique: true},
				},
			},
		},
	}
	meta.InitAliases()
	svc := newFailingService(t, meta, 1, func(s store.Store) {
		addEntity(s, "BUG-001", "bug", map[string]any{"slug": "a"})
		addEntity(s, "BUG-002", "bug", map[string]any{"slug": "b"})
	})

	_, err := svc.FindUniqueViolations(context.Background(), analysis.Options{})
	if err == nil || !analysis.IsIncompleteScan(err) {
		t.Fatalf("FindUniqueViolations error = %v, want an incomplete-scan error", err)
	}
}

// TestHealthyProject_NoIncompleteScan is the false-positive guard
// (acceptance criterion 3): fail-closed must not become fail-noisy.
func TestHealthyProject_NoIncompleteScan(t *testing.T) {
	meta := validationMeta(t)
	svc := newServiceWith(t, meta, func(s store.Store) {
		addEntity(s, "BUG-001", "bug", map[string]any{"title": "first"})
		addEntity(s, "BUG-002", "bug", map[string]any{"title": "second"})
	})

	result, err := svc.RunValidations(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatalf("healthy project reported an error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("got %d violations, want 0", len(result.Violations))
	}
	if _, err := svc.AnalyzeAll(context.Background(), analysis.Options{}); err != nil {
		t.Fatalf("AnalyzeAll on a healthy project: %v", err)
	}
}

// TestIncompleteScanError_Message keeps the operator-facing text
// informative: it must name the failure and carry the cause.
func TestIncompleteScanError_Message(t *testing.T) {
	err := &analysis.IncompleteScanError{Op: "list entities", EntityType: "bug", Err: errParse}
	msg := err.Error()
	for _, want := range []string{"incomplete scan", "list entities", `"bug"`, "failed to parse frontmatter"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
	if !analysis.IsIncompleteScan(err) {
		t.Error("IsIncompleteScan on a direct IncompleteScanError = false")
	}
	if analysis.IsIncompleteScan(errors.New("unrelated")) {
		t.Error("IsIncompleteScan on an unrelated error = true")
	}
}
