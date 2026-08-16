package dataentry

import (
	"context"
	"fmt"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// capMeta declares a type whose `title` is required, so seeding an entity
// WITHOUT a title yields exactly one properties issue per entity — letting a
// test dial the issue count precisely.
func capMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}},
		},
	}
}

// The cap's boundary is the whole risk: 100 must report in full, 101 must
// report 100 AND say so. An off-by-one either drops a legitimate issue or
// labels a complete list as truncated, and both are silent in production.
func TestAnalyzeProperties_CapBoundary(t *testing.T) {
	for _, tc := range []struct {
		seeded        int
		wantIssues    int
		wantTruncated bool
	}{
		{seeded: 0, wantIssues: 0, wantTruncated: false},
		{seeded: 99, wantIssues: 99, wantTruncated: false},
		{seeded: 100, wantIssues: 100, wantTruncated: false},
		{seeded: 101, wantIssues: 100, wantTruncated: true},
		{seeded: 500, wantIssues: 100, wantTruncated: true},
	} {
		t.Run(fmt.Sprintf("seeded=%d", tc.seeded), func(t *testing.T) {
			g := newFixture()
			meta := capMeta()
			for i := range tc.seeded {
				// No title → one required-property error each.
				g.AddNode(&entity.Entity{
					ID: fmt.Sprintf("D-%05d", i), Type: "doc",
					Properties: map[string]any{},
				})
			}
			svc := newAnalyzeService(t, g, meta)
			section := svc.analyzeProperties(context.Background(), meta)

			if len(section.Issues) != tc.wantIssues {
				t.Errorf("issues: want %d, got %d", tc.wantIssues, len(section.Issues))
			}
			if section.Truncated != tc.wantTruncated {
				t.Errorf("truncated: want %v, got %v", tc.wantTruncated, section.Truncated)
			}
		})
	}
}

// The grouping analyzer caps too, via a different mechanism (it cannot stop
// early), so its boundary is asserted independently rather than assumed to
// follow from the streaming one.
func TestAnalyzeDuplicates_CapBoundary(t *testing.T) {
	for _, tc := range []struct {
		seeded        int
		wantIssues    int
		wantTruncated bool
	}{
		{seeded: 100, wantIssues: 100, wantTruncated: false},
		{seeded: 101, wantIssues: 100, wantTruncated: true},
	} {
		t.Run(fmt.Sprintf("seeded=%d", tc.seeded), func(t *testing.T) {
			g := newFixture()
			meta := capMeta()
			// Every entity shares a title, so all of them are duplicates.
			for i := range tc.seeded {
				g.AddNode(&entity.Entity{
					ID: fmt.Sprintf("D-%05d", i), Type: "doc",
					Properties: map[string]any{"title": "same"},
				})
			}
			svc := newAnalyzeService(t, g, meta)
			section := svc.analyzeDuplicates(context.Background(), meta)

			if len(section.Issues) != tc.wantIssues {
				t.Errorf("issues: want %d, got %d", tc.wantIssues, len(section.Issues))
			}
			if section.Truncated != tc.wantTruncated {
				t.Errorf("truncated: want %v, got %v", tc.wantTruncated, section.Truncated)
			}
		})
	}
}

// Truncation must never be silent: a capped section has to reach the wire
// flagged, or an operator fixing all 100 and seeing 100 again concludes the
// tool is broken.
func TestAnalyzeResult_ReportsTruncatedChecks(t *testing.T) {
	g := newFixture()
	meta := capMeta()
	for i := range 150 {
		g.AddNode(&entity.Entity{
			ID: fmt.Sprintf("D-%05d", i), Type: "doc",
			Properties: map[string]any{},
		})
	}
	svc := newAnalyzeService(t, g, meta)
	result := svc.runAnalysis(context.Background(), meta)

	var truncated []string
	for _, s := range result.Sections {
		if s.Truncated {
			truncated = append(truncated, s.Name)
		}
	}
	if len(truncated) == 0 {
		t.Fatal("expected at least one section to report truncation")
	}
	for _, s := range result.Sections {
		if len(s.Issues) > maxSectionIssues {
			t.Errorf("section %q returned %d issues, above the cap of %d",
				s.Name, len(s.Issues), maxSectionIssues)
		}
	}
}

// A capped run must not scan the whole store: the cap exists to bound WORK,
// not merely output. Asserted by counting rows the analyzer pulled.
func TestAnalyzeProperties_StopsScanningAtCap(t *testing.T) {
	g := newFixture()
	meta := capMeta()
	const seeded = 5000
	for i := range seeded {
		g.AddNode(&entity.Entity{
			ID: fmt.Sprintf("D-%05d", i), Type: "doc",
			Properties: map[string]any{},
		})
	}
	svc := newAnalyzeService(t, g, meta)
	counter := &countingAnalyzeReader{inner: svc.reads}
	svc.reads = counter

	section := svc.analyzeProperties(context.Background(), meta)

	if len(section.Issues) != maxSectionIssues {
		t.Fatalf("expected %d issues, got %d", maxSectionIssues, len(section.Issues))
	}
	// One row past the cap is read to DETECT truncation; anything close to
	// the full 5000 means the early break regressed.
	if counter.rows > maxSectionIssues+2 {
		t.Errorf("scanned %d rows for a %d-issue cap — the analyzer should stop "+
			"once truncation is detected, not drain the store", counter.rows, maxSectionIssues)
	}
}

// countingAnalyzeReader counts rows yielded, so a test can assert the
// analyzer stopped scanning rather than merely trimmed its output.
type countingAnalyzeReader struct {
	inner analyzeReader
	rows  int
}

func (c *countingAnalyzeReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return c.inner.GetEntity(ctx, id)
}

func (c *countingAnalyzeReader) ListEntityHeaders(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	src := c.inner.ListEntityHeaders(ctx, q)
	return func(yield func(store.EntityHeader, error) bool) {
		for h, err := range src {
			c.rows++
			if !yield(h, err) {
				return
			}
		}
	}
}
