package predicate_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// FuzzCompile feeds random byte sequences to Compile and asserts only
// that no input causes a panic. The engine's parser-side recover()
// wrapper (RR-S84L) converts any gopher-lua internal panic into a
// typed *ParseError. CI runs this with -fuzz separately; the seed
// corpus below documents the inputs we definitely care about.
func FuzzCompile(f *testing.F) {
	for _, seed := range []string{
		"",
		"   ",
		"true",
		"false",
		"nil",
		"x == 1",
		"\xef\xbb\xbf",              // bare BOM
		"return false",              // leading-return-statement reject
		"\x00",                      // null byte
		"--[[]]",                    // unterminated long comment
		"--[==[ x ]==]return false", // long-bracket at level 2
		"function()",                // syntactic nonsense
		"\"unterminated string",
		"f(g(h(i(j()))))",
		"a and a and a and a",
		"{[1]=2}",
		"return ((((((true))))))",
		// Shapes mirrored from TestCompile_RecoversParserPanics so
		// the fuzz target seeds the same edges (RR-8VKE).
		"\x00\x00\x00",
		"'unterminated",
		"((((((((((",
		"))))))))))",
		"function function function",
		"a.b.c.d.e.f.g.h.i.j.k.l.m.n.o",
		"and or not and or not",
		"{,,,,,,,,,,,,}",
		"1.2.3.4",
	} {
		f.Add(seed)
	}

	env := predicate.NewEnv()
	if err := env.DeclareVar("x", predicate.NumberType); err != nil {
		f.Fatalf("declare: %v", err)
	}

	f.Fuzz(func(_ *testing.T, src string) {
		// The contract is: Compile MUST NOT panic on any input. It may
		// return an error of any of our error types, or it may return
		// a Program. Anything else (panic, hang) is a failure.
		_, _ = predicate.Compile(env, src)
	})
}

// TestCompile_RecoversParserPanics is a documentation-anchored
// smoke test that pins the recover() discipline (RR-S84L). It
// constructs inputs known to push gopher-lua's parser to its
// edges and asserts none panic. If a future change removes the
// recover wrapper, the first input that panics fails this test.
func TestCompile_RecoversParserPanics(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("x", predicate.NumberType); err != nil {
		t.Fatalf("declare: %v", err)
	}

	// Adversarial inputs. None are required to compile; they just
	// must not panic.
	hostile := []string{
		"",
		"\x00\x00\x00",
		"'unterminated",
		"\"unterminated",
		"--[[ unterminated long comment",
		"((((((((((",
		"))))))))))",
		"function function function",
		"a.b.c.d.e.f.g.h.i.j.k.l.m.n.o",
		"and or not and or not",
		"{,,,,,,,,,,,,}",
		"1.2.3.4",
	}
	for _, src := range hostile {
		t.Run(quote(src), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Compile panicked on input %q: %v", src, r)
				}
			}()
			_, _ = predicate.Compile(env, src)
		})
	}
}

func quote(s string) string {
	if len(s) > 24 {
		return s[:24] + "..."
	}
	return s
}
