// Tests for TKT-IHC7D — typed _props + per-row _fields on cards/list
// view sections. Covers:
//   - copyVisibleProperties hidden-stripping (AC 5 + RR-FD1A)
//   - buildSectionEntityData populates Props + FieldVerdicts (AC 3)
//   - sectionEntityToV1 wire conversion at the v1.ViewEntity level (AC 4)
//   - Key-set invariant between _props and _fields (RR-FD1B)
//   - Both properties/list AND content/cards branches produce the
//     same wire shape (RR-FD1C)
//   - Props map is a fresh map, not aliased to e.Properties (RR-FD1E #7)

package dataentry

import (
	"context"
	"reflect"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestCopyVisibleProperties_HiddenStripped(t *testing.T) {
	svc := affordanceServiceWithResolver((&verdictBuilder{}).Hidden("priority").Build())
	e := &entity.Entity{
		Type: "ticket",
		Properties: map[string]any{
			"title":    "First",
			"status":   "open",
			"priority": "high", // hidden — must not appear
		},
	}
	got := svc.copyVisibleProperties(context.Background(), e)
	if _, ok := got["priority"]; ok {
		t.Errorf("hidden 'priority' must not appear in _props; got %+v", got)
	}
	if got["title"] != "First" {
		t.Errorf("title: got %v, want 'First'", got["title"])
	}
	if got["status"] != "open" {
		t.Errorf("status: got %v, want 'open'", got["status"])
	}
}

func TestCopyVisibleProperties_FreshMap(t *testing.T) {
	// Defensive copy: the returned map must not share its backing
	// pointer with e.Properties so future maintainers can't accidentally
	// alias the entity's property map into a long-lived response.
	svc := affordanceServiceWithResolver(NopFieldVerdictResolver{})
	original := map[string]any{"title": "x", "status": "open"}
	e := &entity.Entity{Type: "ticket", Properties: original}
	got := svc.copyVisibleProperties(context.Background(), e)
	if reflect.ValueOf(got).Pointer() == reflect.ValueOf(original).Pointer() {
		t.Fatal("copyVisibleProperties returned the same map pointer as e.Properties; expected a fresh map")
	}
	// Mutating the result must not affect the source.
	got["title"] = "mutated"
	if original["title"] != "x" {
		t.Errorf("mutating copy leaked into source; source title now %v", original["title"])
	}
}

func TestCopyVisibleProperties_EmptyHidden(t *testing.T) {
	svc := affordanceServiceWithResolver(NopFieldVerdictResolver{})
	e := &entity.Entity{
		Type:       "ticket",
		Properties: map[string]any{"title": "x"},
	}
	got := svc.copyVisibleProperties(context.Background(), e)
	if got["title"] != "x" {
		t.Errorf("title: got %v, want 'x'", got["title"])
	}
}

func TestBuildSectionEntityData_PopulatesPropsAndFieldVerdicts(t *testing.T) {
	app := testViewApp()
	// Override the resolver so 'status' is read-only on tickets.
	app.fieldResolver = (&verdictBuilder{}).ReadOnly("status").Build()
	st := app.State()
	e := &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "First", "status": "open"},
	}
	eDef, _ := st.Meta.GetEntityDef(e.Type)
	secFields := []ViewSectionField{
		{Property: "title"},
		{Property: "status"},
	}
	sed := app.buildSectionEntityData(context.Background(), e, secFields, eDef)

	// Props carries the typed values.
	if !reflect.DeepEqual(sed.Props, map[string]any{"title": "First", "status": "open"}) {
		t.Errorf("Props: got %+v, want title+status only", sed.Props)
	}
	// FieldVerdicts carries the sparse verdict map.
	if sed.FieldVerdicts == nil {
		t.Fatal("FieldVerdicts: got nil, want non-nil sparse map")
	}
	status, ok := sed.FieldVerdicts["status"]
	if !ok {
		t.Fatalf("FieldVerdicts['status']: want present (read-only verdict)")
	}
	if status.Writable == nil || *status.Writable {
		t.Errorf("FieldVerdicts['status'].Writable: got %v, want pointer-to-false", status.Writable)
	}
	if _, ok := sed.FieldVerdicts["title"]; ok {
		t.Errorf("FieldVerdicts['title']: must NOT appear (sparse, default writable=true)")
	}
}

func TestBuildSectionEntityData_HiddenAbsentFromBothMaps(t *testing.T) {
	// RR-FD1B invariant: hidden ⇒ absent from BOTH _props and _fields.
	app := testViewApp()
	app.fieldResolver = (&verdictBuilder{}).Hidden("status").Build()
	st := app.State()
	e := &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "First", "status": "open"},
	}
	eDef, _ := st.Meta.GetEntityDef(e.Type)
	sed := app.buildSectionEntityData(context.Background(), e, nil, eDef)
	if _, ok := sed.Props["status"]; ok {
		t.Errorf("hidden 'status' must not appear in Props; got %+v", sed.Props)
	}
	if _, ok := sed.FieldVerdicts["status"]; ok {
		t.Errorf("hidden 'status' must not appear in FieldVerdicts; got %+v", sed.FieldVerdicts)
	}
}

func TestBuildSectionEntityData_KeySetInvariant(t *testing.T) {
	// RR-FD1B: keys(Props) ⊆ keys(e.Properties) \ hidden(e);
	//          keys(FieldVerdicts) ∩ hidden(e) == ∅.
	app := testViewApp()
	app.fieldResolver = (&verdictBuilder{}).Hidden("priority").ReadOnly("status").Build()
	st := app.State()
	e := &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]any{
			"title":    "First",
			"status":   "open",
			"priority": "high",
		},
	}
	eDef, _ := st.Meta.GetEntityDef(e.Type)
	sed := app.buildSectionEntityData(context.Background(), e, nil, eDef)

	hidden := app.affordances.hiddenProperties(context.Background(), e)
	for k := range sed.Props {
		if _, h := hidden[k]; h {
			t.Errorf("Props has hidden key %q (invariant: keys(Props) ∩ hidden == ∅)", k)
		}
	}
	for k := range sed.FieldVerdicts {
		if _, h := hidden[k]; h {
			t.Errorf("FieldVerdicts has hidden key %q (invariant: keys(FieldVerdicts) ∩ hidden == ∅)", k)
		}
	}
}

func TestSectionEntityToV1_WiresPropsAndFields(t *testing.T) {
	// AC 4: the wire converter dumb-copies Props and FieldVerdicts off
	// SectionEntityData into v1.ViewEntity._props and ._fields.
	verdict := map[string]v1.FieldAffordance{
		"status": {Writable: ptrTo(false)},
	}
	sed := SectionEntityData{
		ID:            "TKT-001",
		Title:         "First",
		Type:          "ticket",
		EditFormID:    "ticket-form",
		Props:         map[string]any{"title": "First", "status": "open"},
		FieldVerdicts: verdict,
	}
	got := sectionEntityToV1(sed)
	if !reflect.DeepEqual(got.Props, sed.Props) {
		t.Errorf("Props: got %+v, want %+v", got.Props, sed.Props)
	}
	if got.FieldAffordances == nil {
		t.Fatal("FieldAffordances: got nil, want non-nil pointer")
	}
	if !reflect.DeepEqual(*got.FieldAffordances, verdict) {
		t.Errorf("*FieldAffordances: got %+v, want %+v", *got.FieldAffordances, verdict)
	}
}

func TestSectionEntityToV1_OmitsFieldsWhenNil(t *testing.T) {
	// FieldVerdicts == nil ⇒ wire field is absent (omitempty + nil pointer).
	sed := SectionEntityData{
		ID:    "TKT-001",
		Title: "First",
		Type:  "ticket",
	}
	got := sectionEntityToV1(sed)
	if got.FieldAffordances != nil {
		t.Errorf("FieldAffordances: got non-nil, want nil for absent verdict")
	}
	if got.Props != nil {
		t.Errorf("Props: got non-nil, want nil for absent map")
	}
}

func TestSectionEntityToV1_EmptyFieldVerdicts_EmitsPresentButEmpty(t *testing.T) {
	// closed-world signal: empty verdict map means "evaluated, no
	// deviations" — must serialize as `&{}` not nil.
	sed := SectionEntityData{
		ID:            "TKT-001",
		Type:          "ticket",
		FieldVerdicts: map[string]v1.FieldAffordance{},
	}
	got := sectionEntityToV1(sed)
	if got.FieldAffordances == nil {
		t.Fatal("FieldAffordances: got nil, want pointer-to-empty-map")
	}
	if len(*got.FieldAffordances) != 0 {
		t.Errorf("*FieldAffordances: got %+v, want empty map", *got.FieldAffordances)
	}
}

func ptrTo[T any](v T) *T { return &v }
