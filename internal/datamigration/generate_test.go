package datamigration

import (
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestGenerate_NilWhenInSync(t *testing.T) {
	cur := metaV1().ShapeProjection()
	d, err := Generate(cur, cur, "", testNow())
	if err != nil || d != nil {
		t.Fatalf("Generate on identical shapes = %v, %v; want nil, nil", d, err)
	}
}

func TestGenerate_NilWhenPurelyAdditive(t *testing.T) {
	m2 := metaV1()
	m2.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
	d, err := Generate(metaV1().ShapeProjection(), m2.ShapeProjection(), "", testNow())
	if err != nil || d != nil {
		t.Fatalf("Generate on additive-only change = %v, %v; want nil, nil", d, err)
	}
}

func TestGenerate_DraftForV1ToV2(t *testing.T) {
	d, err := Generate(metaV1().ShapeProjection(), metaV2().ShapeProjection(), "status rework", testNow())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if d == nil {
		t.Fatalf("no draft generated")
	}
	content := string(d.Content)

	// Rename guess is ACTIVE with a GUESS annotation.
	if !strings.Contains(content, "rename_property: {entity: task, from: status, to: state}") {
		t.Errorf("missing rename guess:\n%s", content)
	}
	if !strings.Contains(content, "GUESS") {
		t.Errorf("rename guess not annotated")
	}
	// Enum replacement produces a map_values stub for the named type's
	// property, positionally paired (open→todo, wip→doing).
	if !strings.Contains(content, "map_values") || !strings.Contains(content, "open: todo") {
		t.Errorf("missing map_values stub:\n%s", content)
	}
	if !strings.Contains(content, "TODO") {
		t.Errorf("map_values stub not annotated TODO")
	}
	// Type change produces a convert.
	if !strings.Contains(content, "convert: {entity: task, property: due, to_type: date}") {
		t.Errorf("missing convert step:\n%s", content)
	}
	// The draft round-trips through the parser (Generate already asserts
	// this, but pin the embedded projections too).
	f := mustParse(t, d.FileName, d.Content)
	if f.FromProjection.Hash() != metaV1().ShapeProjection().Hash() {
		t.Error("draft from_projection wrong")
	}
	if f.ToProjection.Hash() != metaV2().ShapeProjection().Hash() {
		t.Error("draft to_projection wrong")
	}
}

func TestGenerate_DeletionsOnlyCommented(t *testing.T) {
	m2 := metaV1()
	delete(m2.Entities["task"].Properties, "tags")
	delete(m2.Entities, "person")
	d, err := Generate(metaV1().ShapeProjection(), m2.ShapeProjection(), "", testNow())
	if err != nil || d == nil {
		t.Fatalf("Generate: %v, %v", d, err)
	}
	content := string(d.Content)
	if !strings.Contains(content, "# - drop_property: {entity: task, property: tags}") {
		t.Errorf("missing commented drop_property:\n%s", content)
	}
	if !strings.Contains(content, "# - drop_entities: {type: person}") {
		t.Errorf("missing commented drop_entities:\n%s", content)
	}
	// No ACTIVE drop lines.
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- drop_") {
			t.Errorf("active drop step emitted: %s", line)
		}
	}
	// A drift-only draft parses with zero steps and both hashes intact.
	f := mustParse(t, d.FileName, d.Content)
	if len(f.Steps) != 0 {
		t.Errorf("drift-only draft has %d active steps, want 0", len(f.Steps))
	}
}

func TestGenerate_ComputedChangesRecomputeEntityOnce(t *testing.T) {
	from := metaV1()
	from.Entities["task"].Properties["effort"] = metamodel.PropertyDef{Type: "integer"}
	live := metaV1()
	live.Entities["task"].Properties["effort"] = metamodel.PropertyDef{Type: "integer"}
	live.Entities["task"].Properties["doubled"] = metamodel.PropertyDef{
		Type: "integer", Computed: "entity.effort * 2",
	}
	live.Entities["task"].Properties["quadrupled"] = metamodel.PropertyDef{
		Type: "integer", Computed: "entity.doubled * 2",
	}

	d, err := Generate(from.ShapeProjection(), live.ShapeProjection(), "add computed values", testNow())
	if err != nil || d == nil {
		t.Fatalf("Generate: %v, %v", d, err)
	}
	content := string(d.Content)
	if got := strings.Count(content, "recompute_computed: {entity: task}"); got != 1 {
		t.Fatalf("recompute step count = %d, want 1:\n%s", got, content)
	}
	if d.Report.Tier() != metamodel.TierDrift {
		t.Fatalf("computed addition tier = %v, want drift", d.Report.Tier())
	}
	if got := len(mustParse(t, d.FileName, d.Content).Steps); got != 1 {
		t.Fatalf("parsed step count = %d, want 1", got)
	}
}

// Names are timestamp-prefixed, not sequential. Sequential numbering collides
// silently across concurrent branches — two PRs cut from the same commit pick
// the same index and git merges both cleanly (BUG-TY2XQC).
func TestGenerate_NamesAreTimestamped(t *testing.T) {
	d, err := Generate(metaV1().ShapeProjection(), metaV2().ShapeProjection(), "status rework", testNow())
	if err != nil || d == nil {
		t.Fatalf("Generate: %v, %v", d, err)
	}
	if want := "20260919143022-status-rework.yaml"; d.FileName != want {
		t.Errorf("file name = %s, want %s", d.FileName, want)
	}
	if !IsMigrationFileName(d.FileName) {
		t.Errorf("generated name %q would be rejected by the loader", d.FileName)
	}
}

// Two migrations drafted at different times sort in creation order, which is
// what makes a plain name sort the run order.
func TestGenerate_NamesSortChronologically(t *testing.T) {
	first, err := Generate(metaV1().ShapeProjection(), metaV2().ShapeProjection(), "first", testNow())
	if err != nil || first == nil {
		t.Fatalf("Generate: %v, %v", first, err)
	}
	second, err := Generate(metaV1().ShapeProjection(), metaV2().ShapeProjection(), "second",
		testNow().Add(time.Hour))
	if err != nil || second == nil {
		t.Fatalf("Generate: %v, %v", second, err)
	}
	if first.FileName >= second.FileName {
		t.Errorf("names do not sort chronologically: %q should precede %q", first.FileName, second.FileName)
	}
}

// A draft must never carry a from/to hash: that is what made a data-only
// migration unrepresentable.
func TestGenerate_EmitsNoShapeHashes(t *testing.T) {
	d, err := Generate(metaV1().ShapeProjection(), metaV2().ShapeProjection(), "x", testNow())
	if err != nil || d == nil {
		t.Fatalf("Generate: %v, %v", d, err)
	}
	content := string(d.Content)
	for _, key := range []string{"\nfrom:", "\nto:"} {
		if strings.Contains(content, key) {
			t.Errorf("draft still emits %q:\n%s", strings.TrimPrefix(key, "\n"), content)
		}
	}
	// The projections stay: step validation and validateDeltasResolved need them.
	if !strings.Contains(content, "from_projection:") || !strings.Contains(content, "to_projection:") {
		t.Error("draft must still embed both projections")
	}
}

func testNow() time.Time {
	return time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)
}
