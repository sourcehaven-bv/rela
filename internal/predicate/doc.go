// Package predicate is a small, sandboxed expression-evaluation engine
// for boolean predicates over named values and host-registered functions.
//
// The package parses a strict subset of Lua expression syntax via
// gopher-lua's parse package (already vendored for write-path automation),
// walks the AST against a hard allow-list, translates it to a typed
// internal IR, and evaluates the IR against caller-supplied bindings.
//
// # Lifecycle
//
//	env := predicate.NewEnv()
//	env.DeclareVar("entity", predicate.RecordType{
//	    "status": predicate.StringType,
//	})
//	env.DeclareVar("current_user", predicate.RecordType{})
//	env.DeclareFunc("has_role", predicate.FuncSig{
//	    Params: []predicate.Type{predicate.RecordType{}, predicate.StringType},
//	    Return: predicate.BoolType,
//	})
//
//	prog, err := predicate.Compile(env,
//	    `entity.status == 'review' and has_role(current_user, 'reviewer')`)
//	if err != nil { ... }
//
//	b := predicate.NewBindings()
//	b.SetVar("entity", predicate.NewRecord(map[string]predicate.Value{
//	    "status": predicate.NewString("review"),
//	}))
//	b.SetVar("current_user", predicate.NewRecord(map[string]predicate.Value{}))
//	b.SetFunc("has_role", predicate.FuncFunc(
//	    func(ctx context.Context, args []predicate.Value) (predicate.Value, error) {
//	        return predicate.NewBool(true), nil
//	    },
//	))
//
//	v, err := prog.Eval(ctx, b)
//	// v.(predicate.Bool).Bool() == true
//
// # Concurrency
//
// A *Program is immutable after Compile and is safe to Eval concurrently
// from multiple goroutines, each with its own Bindings. The Eval call
// allocates per-invocation visitor state; no caches or memoization live
// on *Program.
//
// # Equality semantics
//
// Predicates use Lua-flavored equality, not Go-flavored. Comparison
// across most type pairs is a compile-time error; the few mixed-type
// pairs that compile follow this table:
//
//	a is     b is     a == b
//	-------- -------- --------------------------
//	nil      nil      true
//	nil      anything false
//	bool     bool     Go ==
//	number   number   float64 == (single numeric type, see below)
//	string   string   byte-equal (incl. null bytes)
//
// Ordered comparisons (<, <=, >, >=) require two numbers or two strings;
// strings compare lexicographically (byte-wise).
//
// # Numeric model
//
// Numbers are a single type backed by float64, matching Lua 5.1 semantics.
// Integer literals (1, 0xFF), float literals (1.0, 1.5e-3), and exponential
// forms (1e10) all parse to the same Number type. Bindings of Go int are
// promoted to float64 at binding time; values outside the 53-bit integer
// range round per IEEE 754.
//
// # Security model
//
// The walker rejects any AST node not on the allow-list (default-reject
// branch on every switch). Per-field invariants are enforced beyond node
// type — e.g. AttrGetExpr.Key must be *StringExpr, rejecting computed
// attribute access entity[expr]. A compile-time depth budget (default 256)
// defends against stack overflow from adversarially nested expressions;
// a per-Eval step budget (default 10_000) defends against runtime
// exhaustion. Neither budget can be disabled, only raised.
//
// The package does no I/O: no file access, no network, no goroutine
// spawning. It is a pure function from (Program, Bindings) to a Value.
package predicate
