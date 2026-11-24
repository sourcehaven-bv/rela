package conditionlint

import (
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func assignMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"todo": {Label: "To-do", IDPrefix: "TD-", Properties: map[string]metamodel.PropertyDef{
				"start_date": {Type: metamodel.PropertyTypeDate},
				"rank":       {Type: metamodel.PropertyTypeInteger},
			}},
		},
	}
}

func TestAssignExprProblems(t *testing.T) {
	tests := []struct {
		name, prop, expr, wantErr string
	}{
		{"today", "start_date", "today()", ""},
		{"nil unsets", "start_date", "nil", ""},
		{"integer", "rank", "3", ""},
		{"syntax", "start_date", "today(", "does not compile"},
		{"reads entity", "start_date", "entity.start_date", "does not compile"},
		{"wrong type", "rank", "'x'", "does not compile"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &dataentryconfig.Config{Lists: map[string]dataentryconfig.List{"l": {
				EntityType: "todo",
				Assign: &dataentryconfig.Assign{Properties: map[string]dataentryconfig.AssignValue{
					tc.prop: {Expr: tc.expr},
				}},
			}}}
			got := strings.Join(assignExprProblems(cfg, assignMeta()), "\n")
			if tc.wantErr == "" && got != "" || tc.wantErr != "" && !strings.Contains(got, tc.wantErr) {
				t.Fatalf("problems %q, want %q", got, tc.wantErr)
			}
		})
	}
}

func TestEvalAssignExpr(t *testing.T) {
	meta := assignMeta()
	now := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	date := meta.Entities["todo"].Properties["start_date"]
	rank := meta.Entities["todo"].Properties["rank"]
	for _, tc := range []struct {
		def  *metamodel.PropertyDef
		expr string
		want any
	}{
		{&date, "today()", "2026-11-24"},
		{&date, "nil", nil},
		{&rank, "3", int64(3)},
	} {
		got, err := EvalAssignExpr(t.Context(), meta, tc.def, tc.expr, now)
		if err != nil || got != tc.want {
			t.Errorf("%s: got %v, %v; want %v", tc.expr, got, err, tc.want)
		}
	}
}
