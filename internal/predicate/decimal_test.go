package predicate_test

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

func decimalEnv(t *testing.T) *predicate.Env {
	t.Helper()
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"price": predicate.DecimalType,
		"tax":   predicate.DecimalType,
		"count": predicate.IntType,
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	return env
}

func dec(t *testing.T, s string) predicate.Value {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("bad test decimal %q", s)
	}
	return predicate.NewDecimal(r)
}

func decimalBindings(t *testing.T, price, tax predicate.Value) *predicate.Bindings {
	t.Helper()
	b := predicate.NewBindings()
	if err := b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
		"price": price, "tax": tax, "count": predicate.NewInt(3),
	})); err != nil {
		t.Fatalf("bind: %v", err)
	}
	return b
}

func TestDecimalComparison(t *testing.T) {
	env := decimalEnv(t)
	tests := []struct {
		name  string
		src   string
		price string
		want  bool
	}{
		{"number literal equal ignores trailing zero", "entity.price == 12.5", "12.50", true},
		{"string literal is exact", "entity.price == '12.50'", "12.5", true},
		{"numeric not lexical", "entity.price > 9", "10", true},
		{"ordered false", "entity.price < 9.99", "9.99", false},
		{"le boundary", "entity.price <= 9.99", "9.99", true},
		{"literal on the left", "0.1 < entity.price", "0.2", true},
		{"negative literal", "entity.price > -1.5", "-1.25", true},
		{"ne", "entity.price ~= 1", "1.0", false},
		{"selection coerces literals", "(entity.price > 5 and 1.5 or 2.5) == 1.5", "6", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := predicate.Compile(env, tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			got, err := prog.Eval(context.Background(), decimalBindings(t, dec(t, tc.price), dec(t, "0")))
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if b, ok := got.(predicate.Bool); !ok || b.Bool() != tc.want {
				t.Fatalf("%q = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

func TestDecimalNilIsNotEqual(t *testing.T) {
	prog, err := predicate.Compile(decimalEnv(t), "entity.price == 1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got, err := prog.Eval(context.Background(), decimalBindings(t, predicate.NewNil(), dec(t, "0")))
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if b, ok := got.(predicate.Bool); !ok || b.Bool() {
		t.Fatalf("nil == 1 = %v, want false", got)
	}
}

func TestDecimalArithmeticIsExact(t *testing.T) {
	env := decimalEnv(t)
	tests := []struct {
		src, price, tax, want string
	}{
		{"entity.price + entity.tax", "0.1", "0.2", "0.3"},
		{"entity.price - entity.tax", "10", "0.01", "9.99"},
		{"entity.price * entity.tax", "19.99", "0.21", "4.1979"},
		{"entity.price * 2", "1.25", "0", "2.5"},
		{"-entity.price", "3.5", "0", "-3.5"},
		{"entity.price + 0", "7", "0", "7"},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			prog, err := predicate.CompileValue(env, tc.src, predicate.ValueProfile(predicate.DecimalType))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := prog.Eval(context.Background(), decimalBindings(t, dec(t, tc.price), dec(t, tc.tax)))
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			d, ok := got.(predicate.Decimal)
			if !ok {
				t.Fatalf("result %T, want Decimal", got)
			}
			if text, ok := d.Text(); !ok || text != tc.want {
				t.Fatalf("%s = %q, want %q", tc.src, text, tc.want)
			}
		})
	}
}

func TestDecimalCompileErrors(t *testing.T) {
	env := decimalEnv(t)
	tests := []struct {
		src, wantErr string
	}{
		{"entity.price / entity.tax", `operator "/" is not available for decimal`},
		{"entity.price % entity.tax", `operator "%" is not available for decimal`},
		{"entity.price + entity.count", "requires matching int, decimal or number operands"},
		{"entity.price == entity.count", "cannot compare decimal with int"},
		{"entity.price == 'abc'", `"abc" is not a decimal number`},
		{"entity.price == '1e3'", `"1e3" is not a decimal number`},
		{"entity.price == true", "cannot compare decimal with bool"},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			expected := predicate.DecimalType
			if strings.Contains(tc.src, "==") {
				expected = predicate.BoolType
			}
			_, err := predicate.CompileValue(env, tc.src, predicate.ValueProfile(expected))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
