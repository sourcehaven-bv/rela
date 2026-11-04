package predicatefns_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

func decimalMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"item": {Properties: map[string]metamodel.PropertyDef{
			"price": {Type: metamodel.PropertyTypeDecimal},
		}},
	}}
}

func TestEvaluator_DecimalExpression(t *testing.T) {
	ev := predicatefns.NewEvaluatorWithClock(decimalMeta(), fixedClock())
	cases := []struct {
		src   string
		price any
		want  bool
	}{
		{"entity.price > 9.5", "10", true},
		{"entity.price > 9.5", "9.50", false},
		{"entity.price == 12.5", "12.50", true},
		{"entity.price == '0.30'", 0.3, true},
		{"entity.price ~= nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			prog, err := ev.Compile("item", tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			props := map[string]any{}
			if tc.price != nil {
				props["price"] = tc.price
			}
			got, err := ev.Matches(context.Background(), prog, "item", "I-1", props)
			if err != nil {
				t.Fatalf("matches: %v", err)
			}
			if got != tc.want {
				t.Fatalf("price=%v: got %v want %v", tc.price, got, tc.want)
			}
		})
	}
}

func TestEvaluator_DecimalFilter(t *testing.T) {
	ev := predicatefns.NewEvaluatorWithClock(decimalMeta(), fixedClock())
	cases := []struct {
		clause string
		price  string
		want   bool
	}{
		{"price>9.5", "10", true},
		{"price=12.5", "12.50", true},
		{"price<=0.1", "0.10", true},
		{"price!=1", "1.0", false},
	}
	for _, tc := range cases {
		t.Run(tc.clause, func(t *testing.T) {
			fs, err := filter.ParseAll([]string{tc.clause})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			prog, err := ev.CompileFilter("item", fs)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := ev.Matches(context.Background(), prog, "item", "I-1", map[string]any{"price": tc.price})
			if err != nil {
				t.Fatalf("matches: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	fs, _ := filter.ParseAll([]string{"price>abc"})
	if _, err := ev.CompileFilter("item", fs); err == nil {
		t.Fatal("expected a non-decimal literal to be refused")
	}
}
