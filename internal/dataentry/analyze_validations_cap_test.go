package dataentry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/validator"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Every reader gatedScriptReader can return re-loads in batches, so
// lateGatedReader's missing-method refusal is unreachable in production.
var (
	_ headerResolver = (*visibility.ScriptReader)(nil)
	_ headerResolver = (*visibility.UnrestrictedReader)(nil)
	_ headerResolver = visibility.DenyReader{}
)

// ruleMeta declares doc with an optional owner and one rule per name, each
// requiring an owner of the docs whose `rule` property names it.
func ruleMeta(rules ...string) *metamodel.Metamodel {
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"doc": {Properties: map[string]metamodel.PropertyDef{
			"title": {Type: "string"}, "owner": {Type: "string"}, "rule": {Type: "string"},
		}, DisplayProperty: "title"},
	}}
	for _, r := range rules {
		meta.Validations = append(meta.Validations, metamodel.ValidationRule{
			Name: r, Description: "Rule " + r, EntityType: "doc",
			When: []string{"rule=" + r}, Then: []string{"owner!="},
		})
	}
	return meta
}

// seedViolators adds n docs that break rule, with ids that sort after prefix.
func seedViolators(g *fixture, prefix, rule string, n int) {
	for i := range n {
		g.AddNode(&entity.Entity{
			ID: fmt.Sprintf("%s-%05d", prefix, i), Type: "doc",
			Properties: map[string]any{"title": "Doc " + prefix, "rule": rule},
		})
	}
}

// The re-load stops at one past the cap, so the boundary is where it can
// break: 100 must report in full, 101 must report 100 and say so, also when
// the violators come from two rules and when one batch holds more than the
// cap.
func TestAnalyzeValidations_CapBoundary(t *testing.T) {
	for _, tc := range []struct {
		name          string
		perRule       []int
		wantIssues    int
		wantTruncated bool
	}{
		{"100 in one rule", []int{100}, 100, false},
		{"101 in one rule", []int{101}, 100, true},
		{"past one batch", []int{resolveBatch + 7}, 100, true},
		{"60 and 40", []int{60, 40}, 100, false},
		{"60 and 41", []int{60, 41}, 100, true},
		{"full after the first rule", []int{101, 5}, 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newFixture()
			var rules []string
			for i, n := range tc.perRule {
				r := fmt.Sprintf("r%d", i)
				rules = append(rules, r)
				seedViolators(g, fmt.Sprintf("D%d", i), r, n)
			}
			meta := ruleMeta(rules...)
			section := newAnalyzeService(t, g, meta).analyzeValidations(context.Background(), meta)
			if len(section.Issues) != tc.wantIssues || section.Truncated != tc.wantTruncated {
				t.Fatalf("issues = %d truncated = %v, want %d and %v",
					len(section.Issues), section.Truncated, tc.wantIssues, tc.wantTruncated)
			}
			for _, iss := range section.Issues {
				if iss.Title == iss.EntityID || iss.EntityType != "doc" {
					t.Fatalf("issue %+v lacks the re-loaded header", iss)
				}
			}
		})
	}
}

// missingReader serves every ref except the ids in hidden, as a gate that
// denies them does.
type missingReader struct {
	analyzeReader
	hidden map[string]bool
}

func (m missingReader) ResolveHeadersErr(
	ctx context.Context, refs []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	out, err := m.analyzeReader.ResolveHeadersErr(ctx, refs)
	for ref := range out {
		if m.hidden[ref.ID] {
			delete(out, ref)
		}
	}
	return out, err
}

// A violator the requester may not read takes no slot: with the first of 101
// hidden, the other 100 fill the section and it is complete.
func TestAnalyzeValidations_HiddenViolatorTakesNoSlot(t *testing.T) {
	g := newFixture()
	seedViolators(g, "D", "r", 101)
	meta := ruleMeta("r")
	svc := newAnalyzeService(t, g, meta)
	svc.reads = missingReader{analyzeReader: svc.reads, hidden: map[string]bool{"D-00000": true}}
	section := svc.analyzeValidations(context.Background(), meta)
	if len(section.Issues) != 100 || section.Truncated {
		t.Fatalf("issues = %d truncated = %v, want 100 and false", len(section.Issues), section.Truncated)
	}
	for _, iss := range section.Issues {
		if iss.EntityID == "D-00000" {
			t.Fatal("a hidden violator was reported")
		}
	}
}

// faultingResolveReader fails every batch re-load.
type faultingResolveReader struct{ analyzeReader }

func (faultingResolveReader) ResolveHeadersErr(
	context.Context, []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	return nil, errors.New("gate down")
}

// faultyForReader fails a batch re-load that holds one of the ids in fail,
// as a gate fault on those rows does.
type faultyForReader struct {
	analyzeReader
	fail map[string]bool
}

func (f faultyForReader) ResolveHeadersErr(
	ctx context.Context, refs []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	for _, r := range refs {
		if f.fail[r.ID] {
			return nil, errors.New("gate down")
		}
	}
	return f.analyzeReader.ResolveHeadersErr(ctx, refs)
}

// A re-load failure names the rule that could not be checked and keeps the
// other rules' results, never the fault's text.
func TestAnalyzeValidations_ResolveFaultNamesTheRule(t *testing.T) {
	g := newFixture()
	seedViolators(g, "A", "broken", 3)
	seedViolators(g, "B", "fine", 2)
	meta := ruleMeta("broken", "fine")
	svc := newAnalyzeService(t, g, meta)
	svc.reads = faultyForReader{analyzeReader: svc.reads, fail: map[string]bool{"A-00000": true}}
	section := svc.analyzeValidations(context.Background(), meta)
	if len(section.Issues) != 3 {
		t.Fatalf("issues = %+v, want one error and two violators", section.Issues)
	}
	iss := section.Issues[0]
	if iss.Title != "broken" || iss.Severity != "error" || iss.EntityID != "" ||
		iss.Message != "Validation rule could not be checked; see the server log" {

		t.Errorf("issue = %+v", iss)
	}
	for _, iss := range section.Issues[1:] {
		if iss.Message != "Rule fine" {
			t.Errorf("issue %+v is not one of rule fine's violators", iss)
		}
	}
}

// failingValidator fails CheckRuleFull for the rules named in fail.
type failingValidator struct {
	validator.Validator
	fail map[string]bool
}

func (f failingValidator) CheckRuleFull(ctx context.Context, rule metamodel.ValidationRule) (validator.RuleResult, error) {
	if f.fail[rule.Name] {
		return validator.RuleResult{}, errors.New("rule exploded")
	}
	return f.Validator.CheckRuleFull(ctx, rule)
}

// A rule that cannot be evaluated is reported by name; the next rule still
// runs.
func TestAnalyzeValidations_RuleErrorNamesTheRule(t *testing.T) {
	g := newFixture()
	seedViolators(g, "A", "broken", 3)
	seedViolators(g, "B", "fine", 2)
	meta := ruleMeta("broken", "fine")
	svc := newAnalyzeService(t, g, meta)
	svc.validator = failingValidator{Validator: svc.validator, fail: map[string]bool{"broken": true}}
	section := svc.analyzeValidations(context.Background(), meta)
	if len(section.Issues) != 3 || section.Issues[0].Title != "broken" ||
		section.Issues[0].Message != "Validation rule could not be checked; see the server log" {

		t.Fatalf("issues = %+v, want rule broken's error, then two violators", section.Issues)
	}
}

// countingResolveReader records the refs of every batch re-load.
type countingResolveReader struct {
	analyzeReader
	batches []int
}

func (c *countingResolveReader) ResolveHeadersErr(
	ctx context.Context, refs []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	c.batches = append(c.batches, len(refs))
	return c.analyzeReader.ResolveHeadersErr(ctx, refs)
}

// The re-load stops at one past the cap: output alone cannot show it, since
// capIssues cuts a longer list to the same 100 issues. A full section makes
// no re-load for a later rule, and one batch serves a rule with more
// violators than a batch holds.
func TestAnalyzeValidations_StopsReloadingAtCap(t *testing.T) {
	for _, tc := range []struct {
		name    string
		perRule []int
		want    []int
	}{
		{"full after the first rule", []int{101, 5}, []int{101}},
		{"past one batch", []int{resolveBatch + 7}, []int{resolveBatch}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newFixture()
			var rules []string
			for i, n := range tc.perRule {
				r := fmt.Sprintf("r%d", i)
				rules = append(rules, r)
				seedViolators(g, fmt.Sprintf("D%d", i), r, n)
			}
			meta := ruleMeta(rules...)
			svc := newAnalyzeService(t, g, meta)
			counter := &countingResolveReader{analyzeReader: svc.reads}
			svc.reads = counter
			svc.analyzeValidations(context.Background(), meta)
			if !slices.Equal(counter.batches, tc.want) {
				t.Errorf("re-load batches = %v, want %v", counter.batches, tc.want)
			}
		})
	}
}

// The orphan re-load stops at one past the cap too.
func TestAnalyzeOrphans_StopsReloadingAtCap(t *testing.T) {
	g := newFixture()
	seedViolators(g, "D", "r", 2*resolveBatch)
	meta := ruleMeta()
	svc := newAnalyzeService(t, g, meta)
	counter := &countingResolveReader{analyzeReader: svc.reads}
	svc.reads = counter
	svc.analyzeOrphans(context.Background(), meta)
	if !slices.Equal(counter.batches, []int{resolveBatch}) {
		t.Errorf("re-load batches = %v, want one of %d", counter.batches, resolveBatch)
	}
}

// An orphan re-load failure reports that the check did not run, never a
// partial list.
func TestAnalyzeOrphans_ResolveFaultReportsOneIssue(t *testing.T) {
	g := newFixture()
	seedViolators(g, "D", "r", 3)
	meta := ruleMeta()
	svc := newAnalyzeService(t, g, meta)
	svc.reads = faultingResolveReader{svc.reads}
	section := svc.analyzeOrphans(context.Background(), meta)
	if len(section.Issues) != 1 || section.Truncated {
		t.Fatalf("issues = %+v truncated = %v, want one", section.Issues, section.Truncated)
	}
	if iss := section.Issues[0]; iss.EntityID != "" || iss.Message != "Orphans could not be checked; see the server log" {
		t.Errorf("issue = %+v", iss)
	}
}

// Orphans stop at one past the cap, also across a batch boundary.
func TestAnalyzeOrphans_CapBoundary(t *testing.T) {
	for _, tc := range []struct {
		seeded        int
		wantIssues    int
		wantTruncated bool
	}{
		{100, 100, false},
		{101, 100, true},
		{resolveBatch + 3, 100, true},
	} {
		t.Run(strconv.Itoa(tc.seeded), func(t *testing.T) {
			g := newFixture()
			seedViolators(g, "D", "r", tc.seeded)
			meta := ruleMeta()
			section := newAnalyzeService(t, g, meta).analyzeOrphans(context.Background(), meta)
			if len(section.Issues) != tc.wantIssues || section.Truncated != tc.wantTruncated {
				t.Fatalf("issues = %d truncated = %v, want %d and %v",
					len(section.Issues), section.Truncated, tc.wantIssues, tc.wantTruncated)
			}
		})
	}
}
