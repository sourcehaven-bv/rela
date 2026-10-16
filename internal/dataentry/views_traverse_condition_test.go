package dataentry

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// conditionViewApp is testViewAppWithMixedTypes with view registered and the
// condition compiler wired, as the server wires it.
func conditionViewApp(t *testing.T, view ViewConfig) *App {
	t.Helper()
	app := testViewAppWithMixedTypes()
	s := *app.State()
	cfg := *s.Cfg
	cfg.Views = map[string]ViewConfig{"bouwblok_view": view}
	s.Cfg = &cfg
	app.schema.Publish(&s)
	if err := app.SetViewConditions(AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		t.Fatal(err)
	}
	return app
}

func partsView(where, condition string) ViewConfig {
	return ViewConfig{
		Entry: ViewEntry{Type: "bouwblok"},
		Traverse: []ViewTraverse{{
			From: "entry", FollowIncoming: "partOfBouwblok", CollectAs: "parts",
			Where: where, Condition: condition,
		}},
	}
}

// TestExecuteViewWithCondition covers a traverse `condition:` over a
// collection of mixed types: FUNC-001 (active), FUNC-002 (draft), UC-001
// (active) and SCEN-001, whose type declares no status.
func TestExecuteViewWithCondition(t *testing.T) {
	cases := []struct {
		name, where, condition string
		want                   []string
	}{
		{"compound: one type in one state", "", "entity.type == 'function' and entity.status == 'active'",
			[]string{"FUNC-001"}},
		{"disjunction over types", "", "entity.type == 'scenario' or (entity.type == 'function' and entity.status ~= 'active')",
			[]string{"FUNC-002", "SCEN-001"}},
		{"type only", "", "entity.type == 'usecase'", []string{"UC-001"}},
		{"negation of a type", "", "not (entity.type == 'function')", []string{"SCEN-001", "UC-001"}},
		// SCEN-001 has no status property at all; it binds nil, and nil
		// differs from 'active'.
		{"not-equal includes a type without the property", "", "entity.status ~= 'active'",
			[]string{"FUNC-002", "SCEN-001"}},
		{"a property another type declares is false, not an error", "", "entity.status == 'draft'",
			[]string{"FUNC-002"}},
		{"ANDed with where", "type = function", "entity.status == 'active'", []string{"FUNC-001"}},
		{"where alone still works", "status = active", "", []string{"FUNC-001", "UC-001"}},
		// where: narrows the types first, so a comparison on a property only
		// functions declare is fine on this step.
		{"where on type narrows the condition's types", "type != scenario", "entity.status < 'b'",
			[]string{"FUNC-001", "UC-001"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view := partsView(tc.where, tc.condition)
			app := conditionViewApp(t, view)
			result, err := app.views.executeView(context.Background(), view, "BOUWBLOK-001", defaultViewWorld())
			if err != nil {
				t.Fatalf("executeView: %v", err)
			}
			got := collectIDs(result.Collections["parts"])
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("parts = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestExecuteViewWithCondition_NoCompilerFails pins the fail-closed answer:
// without a compiled program the step does not skip its condition, which
// would show every traversed entity.
func TestExecuteViewWithCondition_NoCompilerFails(t *testing.T) {
	app := testViewAppWithMixedTypes()
	view := partsView("", "entity.type == 'function'")
	if _, err := app.views.executeView(context.Background(), view, "BOUWBLOK-001", defaultViewWorld()); err == nil {
		t.Fatal("want an error without a condition compiler, got a result")
	}
}

// TestExecuteViewWithCondition_UncompiledFails covers a deployment that
// skipped validation: a condition that does not compile fails the view.
func TestExecuteViewWithCondition_UncompiledFails(t *testing.T) {
	view := partsView("", "entity.nope == 'x'")
	app := conditionViewApp(t, view)
	if _, err := app.views.executeView(context.Background(), view, "BOUWBLOK-001", defaultViewWorld()); err == nil {
		t.Fatal("want an error for a condition that does not compile, got a result")
	}
}

// TestViewMentions_Condition covers a `mentioned_by` step with a condition,
// through the HTTP view endpoint with an ACL: the condition narrows the
// readable backlinks, and REQ-001, which alice cannot read, stays absent.
func TestViewMentions_Condition(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	seedEntity(app, &entity.Entity{ID: "TKT-004", Type: "ticket",
		Properties: map[string]any{"title": "open ticket", "status": "open"}, Content: "`FEAT-001`"})
	view := backlinksView("", ViewSection{Display: "list"})
	view.Traverse[0].Condition = "entity.type == 'ticket' and entity.status == 'open'"
	app.Cfg().Views["fv"] = view
	if err := app.SetViewConditions(AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		t.Fatal(err)
	}

	sec := featureView(t, app, d).Sections[0]
	var got []string
	for _, e := range sec.Entities {
		got = append(got, e.ID)
	}
	if !slices.Equal(got, []string{"TKT-004"}) {
		t.Errorf("mentions = %v, want [TKT-004]", got)
	}
}

// TestViewMentions_ConditionReadsRedactedEntity pins that a traverse
// condition cannot decide membership on a field the reader may not see:
// `status` is hidden, so it reads as nil and TKT-004 is not collected.
func TestViewMentions_ConditionReadsRedactedEntity(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"status": false}}})
	seedEntity(app, &entity.Entity{ID: "TKT-004", Type: "ticket",
		Properties: map[string]any{"title": "open ticket", "status": "open"}, Content: "`FEAT-001`"})
	view := backlinksView("", ViewSection{Display: "list"})
	view.Traverse[0].Condition = "entity.status == 'open'"
	app.Cfg().Views["fv"] = view
	if err := app.SetViewConditions(AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		t.Fatal(err)
	}
	if got := featureView(t, app, d).Sections[0].Entities; len(got) != 0 {
		t.Errorf("a condition on a hidden field collected %d rows, want 0", len(got))
	}
}

// TestExecuteViewWithCondition_StaleProgramFails covers a view config that
// no longer matches the published one (a reload between the two reads): the
// step fails rather than dropping the rows it has no program for.
func TestExecuteViewWithCondition_StaleProgramFails(t *testing.T) {
	app := conditionViewApp(t, partsView("", "entity.type == 'function'"))
	stale := partsView("", "entity.type == 'usecase'")
	if _, err := app.views.executeView(context.Background(), stale, "BOUWBLOK-001", defaultViewWorld()); err == nil {
		t.Fatal("want an error for a condition without a program, got a result")
	}
}
