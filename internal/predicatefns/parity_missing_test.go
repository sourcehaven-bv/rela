package predicatefns_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// TestEmptyMissingParity pins the predicate expressions the Phase-2
// filter->predicate transpiler MUST emit to reproduce internal/filter's
// empty/missing-value contract (RR-S251K). filter (match.go:30-63,
// TestMatchMissingProperty) defines:
//
//	prop=value   -> false when prop is missing/empty  (not equal)
//	prop!=value  -> false when prop is missing/empty  (must NOT match)
//	prop=        -> true  when prop is missing/empty   ("is empty")
//	prop!=       -> false when prop is missing/empty   ("is not empty")
//
// Predicate's RAW semantics differ for the != case: on a declared-but-
// absent field, evalAttr yields nil, so `entity.p ~= 'x'` is TRUE — the
// opposite of filter. The transpiler therefore cannot map filter ops
// 1:1; it must wrap with a presence guard. This test proves the guarded
// forms below give filter-identical verdicts, so the transpiler has a
// verified target to generate.
//
// Mapping the transpiler must use (p = entity.<prop>):
//
//	filter `prop=value`  -> `p ~= nil and p == 'value'`
//	filter `prop!=value` -> `p ~= nil and p ~= 'value'`   (presence-guarded!)
//	filter `prop=`       -> `p == nil or p == ''`
//	filter `prop!=`      -> `p ~= nil and p ~= ''`
func TestEmptyMissingParity(t *testing.T) {
	env := predicate.NewEnv()
	// `title` is declared on the record shape but a binding may omit it
	// (evalAttr returns nil for a declared-but-absent field).
	if err := env.DeclareVar("entity", predicate.RecordType{
		"title": predicate.StringType,
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}

	// present: title="hello"; empty: title=""; missing: title unset.
	bindPresent := func() *predicate.Bindings {
		b := predicate.NewBindings()
		_ = b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
			"title": predicate.NewString("hello"),
		}))
		return b
	}
	bindEmpty := func() *predicate.Bindings {
		b := predicate.NewBindings()
		_ = b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
			"title": predicate.NewString(""),
		}))
		return b
	}
	bindMissing := func() *predicate.Bindings {
		b := predicate.NewBindings()
		_ = b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{}))
		return b
	}

	// Each row: the guarded predicate the transpiler emits, and the
	// expected verdict for present / empty / missing — matching what
	// filter.Match returns for the corresponding `title<op>` filter.
	tests := []struct {
		name                    string
		src                     string
		present, empty, missing bool
	}{
		{
			name:    "prop=value  (title=hello)",
			src:     "entity.title ~= nil and entity.title == 'hello'",
			present: true, empty: false, missing: false,
		},
		{
			name:    "prop!=value (title!=hello, presence-guarded)",
			src:     "entity.title ~= nil and entity.title ~= 'hello'",
			present: false, empty: true, missing: false, // filter: missing !=value -> false
		},
		{
			name:    "prop= (is empty)",
			src:     "entity.title == nil or entity.title == ''",
			present: false, empty: true, missing: true,
		},
		{
			name:    "prop!= (is not empty)",
			src:     "entity.title ~= nil and entity.title ~= ''",
			present: true, empty: false, missing: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := predicate.Compile(env, tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			eval := func(b *predicate.Bindings) bool {
				v, err := prog.Eval(context.Background(), b)
				if err != nil {
					t.Fatalf("eval: %v", err)
				}
				return v.(predicate.Bool).Bool()
			}
			if got := eval(bindPresent()); got != tc.present {
				t.Errorf("present: got %v want %v", got, tc.present)
			}
			if got := eval(bindEmpty()); got != tc.empty {
				t.Errorf("empty: got %v want %v", got, tc.empty)
			}
			if got := eval(bindMissing()); got != tc.missing {
				t.Errorf("missing: got %v want %v", got, tc.missing)
			}
		})
	}
}
