package computed_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/computed"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestEvaluate_DecimalIsExact(t *testing.T) {
	m := meta(map[string]metamodel.PropertyDef{
		"price": {Type: metamodel.PropertyTypeDecimal},
		"qty":   {Type: metamodel.PropertyTypeDecimal},
		"total": {Type: metamodel.PropertyTypeDecimal, Computed: "entity.price * entity.qty"},
	})
	set, err := computed.Compile(m)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	cases := []struct {
		name       string
		price, qty any
		want       string
	}{
		{"cents", "0.10", "3", "0.3"},
		{"yaml float", 0.1, 0.2, "0.02"},
		{"negative", "-1.5", "2", "-3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &entity.Entity{ID: "I-1", Type: "item", Properties: map[string]any{"price": tc.price, "qty": tc.qty}}
			if err := set.Evaluate(context.Background(), e); err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if got := e.Properties["total"]; got != tc.want {
				t.Fatalf("total = %#v, want %q", got, tc.want)
			}
		})
	}
}
