package predicate

import (
	"testing"
	"time"
)

func orderedEnv(t *testing.T) *Env {
	t.Helper()
	env := NewEnv()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(env.DeclareVar("edge", RecordType{
		"valid_until": DateTypeWithLayout("2006-01-02"), "starts": DateType, "kind": StringType, "n": IntType,
	}))
	must(env.DeclareVar("other", RecordType{"valid_until": DateType}))
	must(env.DeclareFunc("today", FuncSig{Return: DateType}))
	must(env.DeclareFunc("now", FuncSig{Return: DateType}))
	must(env.DeclareFunc("shift", FuncSig{Params: []Type{DateType}, Return: DateType}))
	return env
}

func TestOrderedConjunction(t *testing.T) {
	funcs := map[string]bool{"today": true, "now": true}
	tests := []struct {
		name string
		src  string
		want []OrderedComparison // nil: not ok
	}{
		{"one call", "edge.valid_until >= today()",
			[]OrderedComparison{{Attribute: "valid_until", Op: ">=", Func: "today"}}},
		{"flipped", "today() < edge.valid_until",
			[]OrderedComparison{{Attribute: "valid_until", Op: ">", Func: "today"}}},
		{"window", "edge.starts <= now() and edge.valid_until >= today()", []OrderedComparison{
			{Attribute: "starts", Op: "<=", Func: "now"}, {Attribute: "valid_until", Op: ">=", Func: "today"},
		}},
		{"string literal", "edge.kind == 'audit'",
			[]OrderedComparison{{Attribute: "kind", Op: "==", Literal: NewString("audit")}}},
		{"date literal", "edge.valid_until <= '2026-12-31'", []OrderedComparison{
			{Attribute: "valid_until", Op: "<=", Literal: NewDate(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))},
		}},
		{"or", "edge.valid_until >= today() or edge.kind == 'a'", nil},
		{"not", "not (edge.valid_until < today())", nil},
		{"inequality", "edge.kind ~= 'a'", nil},
		{"two attributes", "edge.starts <= edge.valid_until", nil},
		{"other record", "other.valid_until >= today()", nil},
		{"unlisted call", "edge.valid_until >= shift(today())", nil},
		{"number literal", "edge.n >= 3", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := Compile(orderedEnv(t), tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			got, ok := prog.OrderedConjunction("edge", funcs)
			if ok != (tc.want != nil) {
				t.Fatalf("ok = %v, want %v (got %+v)", ok, tc.want != nil, got)
			}
			if !ok {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got {
				g, w := got[i], tc.want[i]
				if g.Attribute != w.Attribute || g.Op != w.Op || g.Func != w.Func || !sameLiteral(g.Literal, w.Literal) {
					t.Errorf("leaf %d = %+v, want %+v", i, g, w)
				}
			}
		})
	}
	if _, ok := (*Program)(nil).OrderedConjunction("edge", funcs); ok {
		t.Error("a nil program has no conjunction")
	}
}

func sameLiteral(a, b Value) bool {
	switch av := a.(type) {
	case nil:
		return b == nil
	case String:
		bv, ok := b.(String)
		return ok && av.String() == bv.String()
	case Date:
		bv, ok := b.(Date)
		return ok && av.Time().Equal(bv.Time())
	}
	return false
}
