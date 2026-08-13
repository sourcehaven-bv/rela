package predicatefns_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

func testNow() time.Time {
	return time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
}

// bindRaw coerces a stored raw value to the predicate Value matching the
// property's declared type — the same shapes the transpiled predicate
// expects (int->Int, date->Date, bool->Bool, list->List, else String).
// Off-type/missing binds Nil. Mirrors affordances' binder so the parity
// test exercises the real binding contract.
func bindRaw(prop metamodel.PropertyDef, raw any) predicate.Value {
	if prop.List {
		elems := []predicate.Value{}
		if xs, ok := raw.([]any); ok {
			for _, e := range xs {
				elems = append(elems, bindScalar(prop, e))
			}
		}
		return predicate.NewList(elems)
	}
	return bindScalar(prop, raw)
}

func bindScalar(prop metamodel.PropertyDef, raw any) predicate.Value {
	if raw == nil {
		return predicate.NewNil()
	}
	switch prop.Type {
	case metamodel.PropertyTypeInteger:
		if s, ok := raw.(string); ok {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return predicate.NewInt(n)
			}
			return predicate.NewNil()
		}
		if n, ok := raw.(int); ok {
			return predicate.NewInt(int64(n))
		}
		return predicate.NewNil()
	case metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime:
		switch v := raw.(type) {
		case time.Time:
			return predicate.NewDate(v)
		case string:
			if tm, err := metamodel.ParseDateValue(v, &prop); err == nil {
				return predicate.NewDate(tm)
			}
		}
		return predicate.NewNil()
	case metamodel.PropertyTypeBoolean:
		if s, ok := raw.(string); ok {
			if s == "true" {
				return predicate.NewBool(true)
			}
			if s == "false" {
				return predicate.NewBool(false)
			}
		}
		return predicate.NewNil()
	default:
		if s, ok := raw.(string); ok {
			return predicate.NewString(s)
		}
		return predicate.NewNil()
	}
}

// transpileMeta is a small metamodel exercising each transpilable type.
func transpileMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"item": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: metamodel.PropertyTypeString},
				"count":  {Type: metamodel.PropertyTypeInteger},
				"active": {Type: metamodel.PropertyTypeBoolean},
				"due":    {Type: metamodel.PropertyTypeDate},
				"tags":   {Type: metamodel.PropertyTypeString, List: true},
			}},
		},
	}
}

// TestFromFilter_ParityWithFilterMatch is the AC2 gate: for a matrix of
// filter expressions and stored values, the transpiled predicate must
// yield the SAME verdict as filter.Match. Divergence here would silently
// change --where / automation / validation outcomes.
func TestFromFilter_ParityWithFilterMatch(t *testing.T) {
	t.Parallel()
	meta := transpileMeta()
	def := meta.Entities["item"]

	// Each record is the stored property map for one "item".
	records := []map[string]any{
		{"title": "hello", "count": "10", "active": "true", "due": "2026-01-15", "tags": []any{"a", "b"}},
		{"title": "world", "count": "3", "active": "false", "due": "2026-09-01", "tags": []any{"x"}},
		{}, // all missing
		{"title": "", "count": "", "tags": []any{}}, // present-but-empty
	}

	filters := []string{
		// string equality / inequality / empty / present
		"title=hello", "title!=hello", "title=", "title!=",
		// glob
		"title=hel*", "title!=hel*",
		// integer ordered + equality (numeric, not lexicographic)
		"count>5", "count<5", "count>=10", "count=10", "count!=10",
		// boolean
		"active=true", "active!=true",
		// date ordered
		"due<2026-06-01", "due>=2026-06-01",
		// list any / none
		"tags=a", "tags!=a", "tags=z",
	}

	for _, fexpr := range filters {
		f, err := filter.Parse(fexpr)
		if err != nil {
			t.Fatalf("filter.Parse(%q): %v", fexpr, err)
		}
		src, err := predicatefns.FromFilter(meta, &def, f)
		if err != nil {
			t.Fatalf("FromFilter(%q): %v", fexpr, err)
		}
		env := predicate.NewEnv()
		if derr := env.DeclareVar("entity", predicatefns.EntityRecordType(meta, &def)); derr != nil {
			t.Fatalf("declare: %v", derr)
		}
		if derr := predicatefns.Declare(env); derr != nil {
			t.Fatalf("declare fns: %v", derr)
		}
		prog, cerr := predicate.Compile(env, src)
		if cerr != nil {
			t.Fatalf("compile %q (from %q): %v", src, fexpr, cerr)
		}

		for ri, rec := range records {
			want := filterVerdict(t, meta, "item", f, rec)
			got := predicateVerdict(t, prog, &def, rec)
			if got != want {
				t.Errorf("filter %q record#%d %v: predicate=%v filter=%v\n  src: %s",
					fexpr, ri, rec, got, want, src)
			}
		}
	}
}

// filterVerdict runs filter.Match for one filter against one record.
func filterVerdict(t *testing.T, meta *metamodel.Metamodel, typ string, f *filter.Filter, rec map[string]any) bool {
	t.Helper()
	def := meta.Entities[typ]
	pd := def.Properties[f.Property]
	r := filter.Record{Type: typ, Properties: rec}
	ok, err := filter.Match(r, f, &pd, meta)
	if err != nil {
		// filter errored (e.g. bad value) — treat as non-match, same as
		// the match paths do for our corpus.
		return false
	}
	return ok
}

// predicateVerdict binds the record and evaluates the compiled program.
func predicateVerdict(t *testing.T, prog *predicate.Program, def *metamodel.EntityDef, rec map[string]any) bool {
	t.Helper()
	b := predicate.NewBindings()
	fields := map[string]predicate.Value{}
	for name, prop := range def.Properties {
		fields[name] = bindRaw(prop, rec[name])
	}
	if err := b.SetVar("entity", predicate.NewRecord(fields)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := predicatefns.Bind(b, testNow()); err != nil {
		t.Fatalf("bind fns: %v", err)
	}
	v, err := prog.Eval(context.Background(), b)
	if err != nil {
		return false // eval error = non-match (fail-soft), same as filter
	}
	return v.(predicate.Bool).Bool()
}

// TestFromFilter_UnsupportedRejected pins RR-NKWJS6: cases with no
// faithful predicate equivalent must ERROR at transpile, never emit a
// silently-different predicate.
func TestFromFilter_UnsupportedRejected(t *testing.T) {
	t.Parallel()
	meta := transpileMeta()
	def := meta.Entities["item"]
	cases := []string{
		"missingprop=x",  // unknown property
		"count=notanint", // non-integer literal on int field
		"active=maybe",   // non-boolean literal on bool field
		"tags>x",         // ordered op on a list
		"title~foo*",     // fuzzy-with-wildcard (two-phase, no equivalent)
	}
	for _, expr := range cases {
		f, err := filter.Parse(expr)
		if err != nil {
			// Some invalids may fail at Parse — that's an acceptable rejection.
			continue
		}
		if _, err := predicatefns.FromFilter(meta, &def, f); err == nil {
			t.Errorf("FromFilter(%q): expected transpile error, got nil", expr)
		}
	}
}

// TestFromFilter_ValueEscaping is the RR-TQEHO4 gate: a value containing
// predicate/Lua metacharacters (quote, backslash, newline) must be
// escaped so the transpiled source still Compiles and compares against
// the literal value — never breaks out into injected syntax.
func TestFromFilter_ValueEscaping(t *testing.T) {
	t.Parallel()
	meta := transpileMeta()
	def := meta.Entities["item"]

	values := []string{
		`Bob's ticket`,
		`a\b`,
		"line1\nline2",
		`quote " and ' mixed`,
		`'; return true; --`, // injection attempt
		"tab\there",
	}
	for _, val := range values {
		f, err := filter.Parse("title=" + val)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		src, err := predicatefns.FromFilter(meta, &def, f)
		if err != nil {
			t.Fatalf("FromFilter(title=%q): %v", val, err)
		}
		env := predicate.NewEnv()
		_ = env.DeclareVar("entity", predicatefns.EntityRecordType(meta, &def))
		_ = predicatefns.Declare(env)
		prog, cerr := predicate.Compile(env, src)
		if cerr != nil {
			t.Fatalf("compile of escaped source failed (injection/escaping bug) for %q:\n  src: %s\n  err: %v", val, src, cerr)
		}
		// The predicate must match exactly the stored value equal to val,
		// and not match a different value.
		if got := predicateVerdict(t, prog, &def, map[string]any{"title": val}); !got {
			t.Errorf("value %q: predicate should match the identical stored value; src: %s", val, src)
		}
		if got := predicateVerdict(t, prog, &def, map[string]any{"title": "other"}); got {
			t.Errorf("value %q: predicate should NOT match a different value; src: %s", val, src)
		}
	}
}
