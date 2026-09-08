package predicatefns_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// evalSHA256 compiles and evaluates sha256(x) with `x` bound to in, returning
// the string result.
func evalSHA256(t *testing.T, in string) string {
	t.Helper()
	env := predicate.NewEnv()
	if err := env.DeclareVar("x", predicate.StringType); err != nil {
		t.Fatalf("declare var: %v", err)
	}
	if err := predicatefns.Declare(env); err != nil {
		t.Fatalf("declare fns: %v", err)
	}
	prog, err := predicate.CompileValue(env, "sha256(x)", predicate.ValueProfile(predicate.StringType))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	b := predicate.NewBindings()
	if bindErr := predicatefns.Bind(b, time.Now()); bindErr != nil {
		t.Fatalf("bind: %v", bindErr)
	}
	if setErr := b.SetVar("x", predicate.NewString(in)); setErr != nil {
		t.Fatalf("set var: %v", setErr)
	}
	v, evalErr := prog.Eval(context.Background(), b)
	if evalErr != nil {
		t.Fatalf("eval: %v", evalErr)
	}
	s, ok := v.(predicate.String)
	if !ok {
		t.Fatalf("sha256 returned %T, want predicate.String", v)
	}
	return s.String()
}

// TestSHA256_KnownVectors pins the ENCODING as lowercase hex against published
// NIST/RFC vectors. This is the load-bearing assertion of the whole function:
// a computed sha256 property is stored, indexed and usually `unique:`, so
// changing the encoding would invalidate every derived key already persisted in
// every project. These vectors are the tripwire against that.
func TestSHA256_KnownVectors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty string",
			in:   "",
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "abc",
			in:   "abc",
			want: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name: "composite key shape",
			in:   "web01.example.com/http",
			want: "b7a027ffdd51c19e22e7b7f00c894ef2f3d968896fb40e382df73339c8c645ac",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evalSHA256(t, tc.in)
			if got != tc.want {
				t.Errorf("sha256(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSHA256_LowercaseHexShape asserts the structural properties the encoding
// choice guarantees, independent of any single vector: 64 characters drawn only
// from [0-9a-f]. A base64 digest would fail both, so this catches an encoding
// swap even if someone updated the vectors above to match.
func TestSHA256_LowercaseHexShape(t *testing.T) {
	got := evalSHA256(t, "any input at all")
	if len(got) != 64 {
		t.Errorf("digest length = %d, want 64 (lowercase hex of SHA-256)", len(got))
	}
	for i, r := range got {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			t.Errorf("digest[%d] = %q, want lowercase hex; full digest %q", i, r, got)
			break
		}
	}
}

// TestSHA256_Deterministic pins that the function is pure: the same input
// yields the same digest across evaluations. Computed properties are
// re-evaluated on every write, so a non-deterministic key would silently break
// find-or-create by minting a new identity per delivery.
func TestSHA256_Deterministic(t *testing.T) {
	const in = "host/service"
	first := evalSHA256(t, in)
	second := evalSHA256(t, in)
	if first != second {
		t.Errorf("sha256 is not deterministic: %q then %q", first, second)
	}
}

// TestSHA256_ConcatComposition covers the shape the ticket's Identity section
// specifies — hashing a `..`-joined composite of two entity properties. The
// separator matters: it is what stops ("ab","c") and ("a","bc") colliding.
func TestSHA256_ConcatComposition(t *testing.T) {
	env := predicate.NewEnv()
	for _, v := range []string{"a", "b"} {
		if err := env.DeclareVar(v, predicate.StringType); err != nil {
			t.Fatalf("declare %s: %v", v, err)
		}
	}
	if err := predicatefns.Declare(env); err != nil {
		t.Fatalf("declare fns: %v", err)
	}
	prog, err := predicate.CompileValue(env, `sha256(a .. "/" .. b)`, predicate.ValueProfile(predicate.StringType))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	digest := func(av, bv string) string {
		t.Helper()
		bind := predicate.NewBindings()
		if err := predicatefns.Bind(bind, time.Now()); err != nil {
			t.Fatalf("bind: %v", err)
		}
		if err := bind.SetVar("a", predicate.NewString(av)); err != nil {
			t.Fatalf("set a: %v", err)
		}
		if err := bind.SetVar("b", predicate.NewString(bv)); err != nil {
			t.Fatalf("set b: %v", err)
		}
		v, evalErr := prog.Eval(context.Background(), bind)
		if evalErr != nil {
			t.Fatalf("eval: %v", evalErr)
		}
		return v.(predicate.String).String()
	}

	if got, want := digest("web01", "http"), evalSHA256(t, "web01/http"); got != want {
		t.Errorf("concat composition = %q, want %q", got, want)
	}
	// The separator is what makes the composite injective for these inputs.
	if digest("ab", "c") == digest("a", "bc") {
		t.Error("separator failed to disambiguate (ab,c) from (a,bc)")
	}
}
