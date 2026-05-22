package predicate_test

import (
	"context"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// TestProgram_Eval_Concurrent verifies the documented invariant:
// a *Program is safe for concurrent Eval from multiple goroutines,
// each with distinct Bindings. Must be run under `-race` to be
// meaningful; the unconditional invocation here at least exercises
// the happy path concurrently to catch a regression where someone
// mutates *Program from Eval.
//
// Acceptance criterion: AC9 (RR-BUL2).
func TestProgram_Eval_Concurrent(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("v", predicate.NumberType); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "v < 100")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	const goroutines = 32
	const iter = 200
	var wg sync.WaitGroup
	ctx := context.Background()
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iter {
				value := float64((g*iter + i) % 200)
				b := predicate.NewBindings()
				if err := b.SetVar("v", predicate.NewNumber(value)); err != nil {
					t.Errorf("SetVar: %v", err)
					return
				}
				v, err := prog.Eval(ctx, b)
				if err != nil {
					t.Errorf("goroutine %d iter %d: Eval: %v", g, i, err)
					return
				}
				want := value < 100
				got := v.(predicate.Bool).Bool()
				if got != want {
					t.Errorf("goroutine %d iter %d: Eval(%v) = %v, want %v", g, i, value, got, want)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestProgram_Eval_Concurrent_Complex (M-bundle): runs a more complex
// program in parallel to catch a regression where someone caches sig
// lookup or visitor state on a callNode.
func TestProgram_Eval_Concurrent_Complex(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{
		"status": predicate.StringType,
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	if err := env.DeclareVar("current_user", predicate.RecordType{}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	if err := env.DeclareFunc("has_role", predicate.FuncSig{
		Params: []predicate.Type{predicate.RecordType{}, predicate.StringType},
		Return: predicate.BoolType,
	}); err != nil {
		t.Fatalf("declare: %v", err)
	}
	prog, err := predicate.Compile(env, "has_role(current_user, 'admin') and entity.status == 'ready'")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	const goroutines = 16
	const iter = 100
	var wg sync.WaitGroup
	ctx := context.Background()
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iter {
				isAdmin := (g+i)%2 == 0
				status := "ready"
				if i%3 == 0 {
					status = "in-progress"
				}
				b := predicate.NewBindings()
				_ = b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
					"status": predicate.NewString(status),
				}))
				_ = b.SetVar("current_user", predicate.NewRecord(map[string]predicate.Value{}))
				_ = b.SetFunc("has_role", predicate.FuncFunc(
					func(_ context.Context, _ []predicate.Value) (predicate.Value, error) {
						return predicate.NewBool(isAdmin), nil
					},
				))
				v, err := prog.Eval(ctx, b)
				if err != nil {
					t.Errorf("goroutine %d iter %d: Eval: %v", g, i, err)
					return
				}
				want := isAdmin && status == "ready"
				if v.(predicate.Bool).Bool() != want {
					t.Errorf("goroutine %d iter %d: got %v, want %v (admin=%v status=%s)",
						g, i, v.(predicate.Bool).Bool(), want, isAdmin, status)
					return
				}
			}
		}()
	}
	wg.Wait()
}
