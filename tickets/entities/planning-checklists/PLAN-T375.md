---
id: PLAN-T375
type: planning-checklist
title: 'Planning: Predicate language: gopher-lua expression subset for declarative conditions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope (this PR):**
- New package `internal/predicate/` with no imports from
  `internal/acl`, `internal/dataentry`, `internal/entitymanager`,
  `internal/store`, `internal/entity`, `internal/metamodel`,
  `internal/lua`, `internal/search`, `internal/tracer` (RR-T4CW (d)).
- Public API (everything else is package-private):
  - `func Compile(env *Env, source string) (*Program, error)` —
    parses one Lua expression, walks AST against allow-list,
    resolves symbols against env, returns reusable Program.
    **Returns `*CompileError` if env is nil** (RR-UJW6).
  - `type Env struct { ... }` + `NewEnv()`, `(*Env).DeclareVar(name string, t Type)`,
    `(*Env).DeclareFunc(name string, sig FuncSig)`. Reject duplicate
    declarations.
  - `type Program struct { ... }` — opaque, immutable after
    Compile. **Safe for concurrent Eval** (RR-BUL2).
  - `func (*Program) Eval(b Bindings, opts ...EvalOption) (Value, error)` —
    per-call options (RR-UJW6); per-call visitor state for
    concurrency safety.
  - `type EvalOption` — `WithStepBudget(n int)`, default 10k.
  - `type Bindings struct { Vars map[string]Value; Funcs map[string]Func }` —
    runtime values + host-fn implementations.
  - `type Value interface { ... }` — sum type, sealed via
    package-private method. Variants: `Bool`, `Number`, `String`,
    `Nil`, `Record`, `List`.
  - `type Func func(args []Value) (Value, error)` — host function.
  - `type ParseError struct { Line, Col int; Msg string }`,
    `type CompileError struct { Line, Col int; Reason string }`,
    `type EvalError struct { Reason string }`.
  - `func LintAll(env *Env, sources []NamedSource) []Issue` —
    batch helper.
- Step-budget enforcement inside Eval (configurable via
  `WithStepBudget`, default 10k).
- **Compile-time depth budget** (RR-XKNO) default 256, configurable
  via a `CompileOption` for symmetry. Defends against stack
  overflow on deeply-nested ASTs.
- Fuzz harness (`go test -fuzz`) over Compile.

**Out of scope (this PR):**
- Wiring into `internal/acl` or `acl.yaml` (a follow-up ticket).
- Domain-specific host functions like `has_relation`,
  `count_relations`, `has_role`. We ship the *machinery* that lets
  callers register such functions; the ACL caller defines them.
  This PR's tests register a couple of toy host functions to
  exercise the API.
- Temporal facts (`now()`, durations) — deferred per design doc.
- SPA/wire/transport changes.
- Documentation under `docs/` (will land with the ACL integration
  that has a story to tell users).

**Acceptance Criteria:**

1. **AC1 — Compile accepts the full worked-use-case corpus.** Given
   an env declaring `entity (record)`, `current_user (record)`, `env (record)`,
   `has_role (string)->bool`, `has_relation (string, record)->bool`,
   `count_relations (string)->number`, `is_one_of (any, ...any)->bool`,
   `contains (list, any)->bool`, `Compile` succeeds for every
   expression in `predicate/testdata/accept/*.lua`. **The corpus has
   ≥15 files** (RR-T4CW (e)) covering at least one example from each
   of the six shapes in the design doc.

   Test: `TestCompile_AcceptsValidExpressions`, table-driven.

2. **AC2 — Compile rejects every disallowed construct.** For each
   banned construct (function literal, table method, length operator,
   string concat, arithmetic, unary minus, varargs, bracket attribute
   access, computed table key, nested call in table arg, multi-stmt
   source, multi-return-value, source starting with `return`,
   non-Return top-level stmt), `Compile` returns an error whose
   message names the construct (RR-7VJJ, RR-8GOP).

   Test: `TestCompile_RejectsDisallowedConstructs`, table-driven
   over `predicate/testdata/reject/*.lua`.

3. **AC3 — Compile rejects unknown symbols against env.** Unknown
   variable, unknown function, unknown attribute, **and nil env**
   (RR-UJW6) are all compile-time errors.

   Test: `TestCompile_RejectsUnknownSymbols`,
   `TestCompile_RejectsNilEnv`.

4. **AC4 — Eval returns the expected value for representative
   rules.** Five end-to-end scenarios mirroring use cases 1.1, 2.1,
   3.1, 4.2, 6.1 (entity property, current_user match, boolean
   composition, function returning number, env global). Each
   compiles and evaluates to the expected value for two binding
   sets (one true, one false). **A worked example sits inline in
   doc.go and the AC4 test source** (RR-T4CW (a)) so the contract
   is visible without cross-referencing.

   Test: `TestProgram_Eval_EndToEnd`.

5. **AC5 — Step budget aborts at eval time, depth budget at compile
   time** (RR-XKNO). Eval test forces a chained-`and` IR exceeding
   the step budget; compile test feeds 1024 nested parens and
   expects a `*CompileError`, not a panic.

   Tests: `TestProgram_Eval_StepBudget`, `TestCompile_RejectsDeeplyNestedExpression`.

6. **AC6 — Fuzz harness, parse-panic recovery.** `FuzzCompile`
   exists; `go test -fuzz=Fuzz -fuzztime=5s` runs cleanly.
   `TestCompile_RecoversParserPanics` asserts the recover wrapper
   is in place via a deliberately panicky `io.Reader` substitute
   for `parse.Parse` (RR-S84L) — guards against accidental removal.

   Tests: `FuzzCompile`, `TestCompile_RecoversParserPanics`.

7. **AC7 — Lint helper.** `LintAll(env, sources)` batch-compiles;
   for a slice of N sources with K invalid, returns exactly K
   `Issue{Name, Err}` records in source order.

   Test: `TestLintAll_ReportsAllSourceErrors`.

8. **AC8 — Package boundary** (extended per RR-T4CW (d)).
   `internal/predicate` does not import `internal/acl`,
   `internal/dataentry`, `internal/entitymanager`,
   `internal/store`, `internal/entity`, `internal/metamodel`,
   **`internal/lua`**, `internal/search`, or `internal/tracer`.

   Test: arch-lint config + `TestPackageImports` reading the
   package's own imports list.

9. **AC9 — Concurrency safety** (RR-BUL2). A single `*Program`
   evaluated from N goroutines (with distinct `Bindings` each)
   produces the expected result with no race-detector violations.

   Test: `TestProgram_Eval_Concurrent`, run under `-race`.

10. **AC10 — Numeric and equality semantics** (RR-VI93, RR-POA2).
    `1`, `1.0`, `0xFF`, `1e10`, `1.5e-3` all compile to `Number`
    values; `nil == nil` is true, `nil == false` is false, `nil`
    compared to any other type is false; string equality is
    byte-equal including null bytes.

    Tests: `TestCompile_NumberLexicalForms`, `TestEval_EqualitySemantics`.

11. **AC11 — Source preprocessing.** A leading UTF-8 BOM is
    stripped; sources starting with `return` (after whitespace,
    comments, and BOM strip) are rejected with a named error.

    Tests: `TestCompile_StripsBOM`, `TestCompile_RejectsLeadingReturn`.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **CEL (`google.golang.org/cel-go`)** — considered and rejected.
  Adds ~9.5 MB to the binary, second syntax for contributors, AST
  shape differs from Lua (rewrite layer needed for `has_relation`
  ergonomics). Detailed comparison in
  `.ignored/cel-vs-expr-comparison.md`.
- **expr-lang/expr** — considered and rejected. Silent missing-key
  semantics (`entity.misspelld` → `false` instead of an error) is
  the wrong default for an authorization predicate; two High-sev
  CVEs in the last 18 months.
- **EDN custom** — considered and rejected. Requires a new ~200-line
  reader; loses the "syntax already familiar to contributors" win.
- **gopher-lua's `parse.Parse`** — chosen. Module is already in the
  binary (`go.mod` declares `github.com/yuin/gopher-lua v1.1.2`).
  Entry point: `parse.Parse(io.Reader, string) ([]ast.Stmt, error)`.
  AST node types declared in
  `github.com/yuin/gopher-lua/ast/{expr,stmt,misc}.go`. The walker
  has to discriminate on these node types — verified the type set
  (about 25 expression node types, plus statement types we reject
  entirely).

**Similar patterns in the codebase:**

- `internal/markdown/parse.go` — small parser-walker style, error
  handling pattern to mirror (return `*ParseError{Line, Col, Msg}`
  not `errors.New(...)`).
- `internal/automation/condition.go` — currently uses a hand-rolled
  string-matching condition format for the `becomes:` clause. **Not
  reusing this** — it has no expressions, only literal-property-
  equality. Different problem.
- `internal/lua/runtime.go` — already constructs a `gopher-lua.LState`
  for the write path. Reuses gopher-lua's *interpreter*. The
  predicate package will reuse only `parse.Parse` (not `LState`)
  because the interpreter is the surface we explicitly want to
  avoid touching.

**Reference implementations:**

- Cerbos's PDP uses CEL with a thin domain-rewrite layer
  (`cerbos.dev/blog/cerbos-policy-language-design`). We're applying
  the same architectural shape (parser + walker + symbol table)
  with a different parser.
- Sanity's conditional-field callbacks
  (`sanity.io/docs/studio/conditional-fields`) explicitly forbid
  async / IO in field predicates. Same constraint here: no I/O,
  pure functions over bindings.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Files to create

```
internal/predicate/
├── doc.go            # Package doc, worked lifecycle example,
│                     # equality semantics table, concurrency
│                     # invariant.
├── env.go            # Env, Type, FuncSig, DeclareVar, DeclareFunc.
├── value.go          # Value interface (sealed) + variants:
│                     # Bool, Number (float64), String, Nil,
│                     # Record, List.
├── bindings.go       # Bindings type.
├── preprocess.go     # BOM strip + leading-token guard +
│                     # whitespace/comment skip.
├── parse.go          # Wraps gopher-lua's parse.Parse; "return "
│                     # prepend + chunk shape validation + recover.
├── compile.go        # Walks AST with allow-list + depth budget;
│                     # produces Program (typed IR, not Lua AST).
├── ir.go             # The Program IR node types.
├── program.go        # Program type, immutable after Compile.
├── eval.go           # Eval loop, step budget, per-call state.
├── lint.go           # LintAll helper.
├── errors.go         # ParseError, CompileError, EvalError.
├── compile_test.go   # AC1, AC2, AC3, AC10, AC11.
├── eval_test.go      # AC4, AC5 (eval side), AC10 eval matrix.
├── concurrent_test.go # AC9.
├── arch_test.go      # AC8.
├── fuzz_test.go      # AC6.
└── testdata/
    ├── accept/*.lua  # ≥15 files, one per use case in design doc.
    └── reject/*.lua  # one per banned construct + per source-shape
                      # error from AC2.
```

### The "expression-only" trick

`parse.Parse` expects a Lua chunk (zero or more statements). To
parse a single expression we prepend `return ` to the source. The
result is a one-statement chunk whose only statement is a
`*ast.ReturnStmt`. We then extract `returnStmt.Exprs[0]` and
discard everything else (length != 1 is a compile error).

Why prepend instead of using `lparse.Exp(...)` directly: `parse.Exp`
is an unexported `yyParser` method and not part of the public API.
Prepending `return ` works against the public `Parse` entry point
and is what most embedders do (gopher-lua's own REPL works
similarly).

### The Program IR

The walker produces a small typed IR (~10 node types), not the Lua
AST directly. This decouples eval from gopher-lua, so:

- Eval is a tight switch over our own node types.
- A future gopher-lua upgrade can't silently widen what eval
  accepts — new AST nodes won't translate into our IR.
- Tests can construct Programs directly without going through Lua
  source.

IR node types:
- `binOp(op, lhs, rhs)` — for `==`, `~=`, `<`, `<=`, `>`, `>=`,
  `and`, `or`.
- `unaryNot(rhs)` — for `not`.
- `varRef(name)` — for an env variable.
- `attrGet(rhs, name)` — for `entity.status`, `current_user.mfa_fresh`.
- `call(fn, args)` — for `has_role(...)`.
- `literal(value)` — for scalars.
- `tableLit(entries)` — for `{status='open'}` named-args.

### Allow-list specifics

The walker's type switch has a **default-reject** branch that
returns `*CompileError{Reason: "unsupported expression kind: %T"}`
for any AST node not explicitly matched. Each accepted node also
asserts **per-field invariants** — accepting the node type is not
the same as accepting all values its fields can hold. This is the
fix for RR-V0OE.

`ast.Expr` implementations to **accept**:

- `IdentExpr` — no field invariants beyond name lookup against env.
- `AttrGetExpr` — **`Key` field must be `*StringExpr`** (the
  dot-sugar form `entity.status`). Reject bracket-indexing
  `entity[expr]` with a named error (`"computed attribute access
  rejected; use dot-sugar only"`).
- `NumberExpr` — see "Numeric type model" below (RR-VI93).
- `StringExpr` — bytes preserved verbatim; null bytes (`\0`) are
  legal (RR-POA2).
- `TrueExpr`, `FalseExpr`, `NilExpr` — pure constants.
- `RelationalOpExpr` — operators must be in {`==`, `~=`, `<`,
  `<=`, `>`, `>=`}. Default-reject on any other operator.
- `LogicalOpExpr` — operators must be in {`and`, `or`}.
- `UnaryNotOpExpr` — `not`.
- `FuncCallExpr` — **only** the `Func + Args` form. **Reject when
  `Receiver != nil`** (the `t:method(...)` form). **Reject when
  `AdjustRet` is true** (multi-value return adjustment; we don't
  support multiple return values).
- `TableExpr` — **only** when it appears as a single argument to a
  `FuncCallExpr`. **Each `Field` entry must be a `*KeyValueField`**
  (reject `*NumberKeyField`, `*NoKeyField`). **`KeyValueField.Key`
  must be `*StringExpr`** (the `{key='value'}` sugar; reject
  `{[expr]=val}`). **`KeyValueField.Value` must be a `ConstExpr`**
  (no nested calls or attribute access inside the named-args
  table).

**Reject (named error per case):**

- `FunctionExpr` (lambda literal)
- `StringConcatOpExpr` (`..`)
- `ArithmeticOpExpr` (`+ - * / % ^`)
- `UnaryMinusOpExpr`, `UnaryLenOpExpr` (`-x`, `#t`)
- `Comma3Expr` (varargs `...`)

The walker also enforces a **depth budget** (default 256, configurable
via `CompileOptions.MaxDepth`) to defend against stack overflow on
adversarial deep nesting like `((((((x))))))` (RR-XKNO). Crossing the
budget returns `*CompileError{Reason: "expression nests too deeply"}`,
never a panic.

### Numeric type model (RR-VI93)

`NumberExpr.Value` is a string token. We follow **Lua 5.1 semantics:
one numeric type** — all numbers are `float64` in the IR.

- `1`, `1.0`, `1e10`, `0xFF` all compile to `Value{kind: number,
  num: 1.0 / 1.0 / 1e10 / 255.0}`.
- Comparison rules: `==`, `<`, `<=`, etc. between two numbers use
  `float64` semantics directly.
- A Go `int(1)` in `Bindings` is converted to `float64(1)` at
  binding time (cheap, exact for 53-bit integer range — beyond
  that the binding errors with a documented loss-of-precision
  message).
- Bindings types: `bool`, `string`, `number` (float64-backed),
  `nil`, `record`, `list[T]`. No separate `int` type.

The trade-off vs Lua 5.3's int/float split: we lose the ability to
express "exactly an integer" — but the ACL use case never needs it
(no modular arithmetic, no integer-overflow concerns). The
Lua-5.1-style single numeric type is the simpler and safer choice.

Pin with `TestCompile_NumberLexicalForms` covering `1`, `1.0`,
`1e10`, `0xFF`, `1.5e-3`, all evaluating to the expected `float64`
in the IR.

### Equality semantics (RR-POA2)

| `a` type | `b` type | `a == b` result                  |
|----------|----------|----------------------------------|
| nil      | nil      | true                              |
| nil      | any other| false                             |
| bool     | bool     | Go `==`                           |
| bool     | any non-nil non-bool | false                 |
| number   | number   | float64 `==`                      |
| string   | string   | Go `==` (byte-equal incl. nulls)  |
| record   | record   | not allowed at compile time      |
| list     | list     | not allowed at compile time      |

`~=` is the negation of `==`. `<`, `<=`, `>`, `>=` allowed only on
two numbers or two strings (lexicographic). Comparing other type
pairs is a **compile-time** type error — caught before eval.

### Source-acceptance contract (RR-7VJJ, RR-8GOP)

After `parse.Parse` returns the chunk:

1. **`len(chunk) == 1`** — exactly one statement. Reject otherwise
   (`"source must contain exactly one expression"`).
2. **`chunk[0]` is `*ast.ReturnStmt`** — the result of our `return `
   prepend. Reject otherwise (`"expected a single expression"`).
3. **`len(returnStmt.Exprs) == 1`** — exactly one expression in
   the return. Reject otherwise (`"multiple return values are not
   supported"`).
4. **Pre-parse: source must not start with `return`** (after
   stripping leading whitespace, comments, and BOM). Reject with
   `"source must be an expression, not a statement"`. This catches
   `return false` becoming `return return false` after the prepend.

### Source preprocessing (RR-T4CW)

Before prepending `return `, the source is preprocessed:

- A leading UTF-8 BOM (`﻿`) is stripped.
- The leading-token check (above) skips ASCII whitespace and Lua
  line/block comments (`-- ...`, `--[[ ... ]]`).
- The source is otherwise passed verbatim. Embedded null bytes
  in string literals are legal.

### The `return` prepend trick

`parse.Parse` expects a Lua chunk. We synthesise:

```go
src := "return " + cleaned  // cleaned = source after BOM strip
stmts, err := parse.Parse(strings.NewReader(src), name)
```

This works against the public parser entry point. If `parse.Parse`
itself panics on adversarial input, a top-level `defer recover()`
in `Compile` converts the panic to `*ParseError` (R3, RR-S84L).

### Type checking at compile time

The Env declares each variable's type (`bool`, `int`, `float`,
`string`, `record`, `list[T]`) and each function's signature
(`(...types) -> type`). The walker propagates types bottom-up and
enforces:

- Operator type rules (`<` requires int/float, `==` allows
  comparing two values of the same type, `and`/`or` require bools).
- Attribute access requires a `record` parent.
- Function call arg types match the declared signature.
- Top-level expression must produce a `bool`.

Type errors at compile time are the missing-key-safety win — the
property "a typo silently flips access" is closed by the
"unknown symbols are a hard compile error" rule, plus the type
checker catches "you compared a string to a bool by accident."

### Eval step budget

Each call to `Eval(...)` increments a counter per IR node visit.
Hard cap default 10_000 — generous for hand-written rules,
catastrophic enough to abort an adversarial deeply-nested
expression. Configurable via `EvalOptions{StepBudget int}`.

### Errors

Three error types:
- `*ParseError{Line, Col, Msg}` — gopher-lua parse failure
  (passed-through line/col).
- `*CompileError{Line, Col, Node, Reason}` — walker rejected a
  node or symbol lookup failed.
- `*EvalError{Reason}` — runtime: type mismatch where the type
  checker conservatively allowed it, or step-budget exhaustion.

### Bindings shape

```go
type Bindings struct {
    Vars      map[string]Value
    Funcs     map[string]Func  // overrides env's host fns
}
type Func func(args []Value) (Value, error)
```

Eval looks up by name; the env guarantees the name exists (compile
check); a missing concrete value at eval time is an `EvalError`
(shouldn't happen if the env declares all names the caller binds).

### Worked example (lifecycle) — for AC4 and doc.go

```go
env := predicate.NewEnv()
env.DeclareVar("entity", predicate.RecordType{
    "status": predicate.StringType,
})
env.DeclareVar("current_user", predicate.RecordType{
    "id": predicate.StringType,
})
env.DeclareFunc("has_role", predicate.FuncSig{
    Params: []predicate.Type{predicate.StringType},
    Return: predicate.BoolType,
})

prog, err := predicate.Compile(env,
    `entity.status == 'review' and has_role('reviewer')`)
if err != nil { ... }

bindings := predicate.Bindings{
    Vars: map[string]predicate.Value{
        "entity": predicate.NewRecord(map[string]predicate.Value{
            "status": predicate.NewString("review"),
        }),
        "current_user": predicate.NewRecord(map[string]predicate.Value{
            "id": predicate.NewString("alice"),
        }),
    },
    Funcs: map[string]predicate.Func{
        "has_role": func(args []predicate.Value) (predicate.Value, error) {
            role := args[0].(predicate.StringValue).String()
            return predicate.NewBool(role == "reviewer"), nil
        },
    },
}

v, err := prog.Eval(bindings)
// v.(predicate.BoolValue).Bool() == true
```

The same `*prog` can be evaluated concurrently from multiple
goroutines with distinct `Bindings` (AC9).

### Test data

`testdata/accept/` contains the worked use cases from the design
doc, ported to a generic env (`entity.status`, `current_user.id`,
`has_role(name)`, `is_one_of(value, ...)`). One `.lua` file per use
case, named after it. Plus a few hand-crafted edge cases:
- expression with deeply-nested parens
- string literals with quotes
- nil comparison
- numeric comparison
- empty table arg

`testdata/reject/` has one file per banned construct, named so the
test can extract the expected reject-reason from the filename.

**Files to modify (outside the new package):**

- `.golangci.yml` — likely nothing; predicate is a normal Go
  package.
- `internal/arch/arch_test.go` (or wherever arch-lint config
  lives) — add an entry forbidding predicate from importing the
  listed domain packages.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Source: Lua expression strings.** Eventually from `acl.yaml`
  (next PR), today only from tests. Validation: gopher-lua's parser
  produces an AST; our walker rejects any node type not in the
  allow-list. **Allowlist, not blocklist** — anything outside the
  ~10 IR-translatable AST types is rejected by the default branch
  of the walker's type switch, not by a list of disallowed types.
  Adding a new AST node upstream (Lua bitwise ops, Lua 5.4 integer
  division) does not silently widen what we accept; the walker
  returns "unsupported expression kind: %T" until we explicitly
  decide.
- **Source: Bindings at eval time.** Values supplied by callers.
  Validation: type-checked against the env signature when the
  caller calls `Eval`. A binding shape mismatch is an `EvalError`,
  not a panic.
- **Source: Host function results.** Functions returning Values
  must respect the declared return type; we check this at call
  boundary. A host fn returning the wrong type is an `EvalError`.

**Security-Sensitive Operations:**

- **None directly.** The predicate package does no I/O, no file
  access, no network, no crypto. It is a pure function from
  (compiled program, bindings) → boolean.
- **The package is *part of* an authorization pipeline** (next PR).
  This puts a structural requirement on the package: **no panics
  on attacker-shaped input.** The fuzz harness is the structural
  defence. Goal: 5 seconds of fuzzing finds no crashes / panics /
  timeouts on a clean tree.
- **Step budget** prevents adversarial expressions from causing
  CPU exhaustion if a future caller compiles user-supplied source.

**Error message safety:**

Error messages name AST node types and operator kinds but never
include caller-supplied binding values. A `CompileError` says
"unsupported expression kind: *ast.FunctionExpr at line 3, col 8" —
no source data. An `EvalError` says "type mismatch: expected bool,
got string" — no value text. This is structural protection against
a future caller logging the error verbatim into an exposed channel.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test                                             | Mechanism                                                  |
|----|--------------------------------------------------|------------------------------------------------------------|
| 1  | `TestCompile_AcceptsValidExpressions`            | Table over `testdata/accept/*.lua` (≥15 files; one per use case across all six shapes). |
| 2  | `TestCompile_RejectsDisallowedConstructs`        | Table over `testdata/reject/*.lua`; expect substring match in error. |
| 3  | `TestCompile_RejectsUnknownSymbols` + `TestCompile_RejectsNilEnv` | Unknown var/func/attr + nil env. |
| 4  | `TestProgram_Eval_EndToEnd`                      | Five end-to-end cases; each evaluated with 2 binding sets. Worked example inline. |
| 5  | `TestProgram_Eval_StepBudget` + `TestCompile_RejectsDeeplyNestedExpression` | Eval budget test; compile-time depth test (1024 nested parens → CompileError not panic). |
| 6  | `FuzzCompile` + `TestCompile_RecoversParserPanics` | Fuzz harness; panicky-Reader test pins the recover wrapper. |
| 7  | `TestLintAll_ReportsAllSourceErrors`             | Slice of 3 sources, 2 invalid; expect 2 ordered issues. |
| 8  | `TestPackageImports` + arch-lint config          | Reads predicate's go-build imports; checks none of the forbidden list (including `internal/lua`). |
| 9  | `TestProgram_Eval_Concurrent` (under `-race`)    | One `*Program`, N goroutines, distinct Bindings; no races, expected results. |
| 10 | `TestCompile_NumberLexicalForms` + `TestEval_EqualitySemantics` | Numeric model + equality matrix. |
| 11 | `TestCompile_StripsBOM` + `TestCompile_RejectsLeadingReturn` | Source preprocessing. |

**Edge Cases:**

- Empty source string → `ParseError` (gopher-lua handles this).
- Whitespace-only source → same.
- Single literal (`true`) — should compile, return bool, eval to
  `true`. Tests: yes.
- Single `nil` — should compile (nil is a value), eval-time error if
  used in a boolean context (caller must wrap in comparison).
- Deeply nested parens — accept; budget should not be wasted on
  parens because parens collapse during walking.
- Boolean composition with `not` chained (`not not not x`) — accept;
  evaluator handles.
- String comparison `==` between literal and var — accept; type
  check OK.
- Comparison between incompatible types (`entity.status == 5` where
  status is string) — compile-time type error.
- Function call with too few/many args — compile-time arity error.
- Function call with wrong arg type — compile-time type error.
- Table-arg with non-string key (`{[expr]=val}`) — reject (keys must
  be strings produced by `key='value'` sugar).
- Table-arg with non-const value (`{x=foo()}`) — reject.
- Method call (`x:method(y)`) — reject explicitly with named error.

**Negative Tests:** (the `testdata/reject/` corpus, one file each)

- `function_literal.lua` → `function() return 1 end`
- `arithmetic.lua` → `1 + 2`
- `string_concat.lua` → `"a" .. "b"`
- `unary_minus.lua` → `-x`
- `length.lua` → `#t`
- `varargs.lua` → `(...)`
- `method_call.lua` → `t:m()`
- `do_block.lua` → `do return 1 end`
- `goto_stmt.lua` → `goto x; ::x::`
- `assignment.lua` → `x = 1`
- `unknown_symbol.lua` → `nonexistent_var`
- `unknown_function.lua` → `mystery_call(1)`
- `wrong_arity.lua` → call with too many args
- `wrong_arg_type.lua` → string passed where bool expected
- `non_bool_top_level.lua` → top-level expression of type string
- `nested_call_in_table_arg.lua` → `{x=foo()}`
- `bracket_attr_access.lua` → `entity['status']` (RR-V0OE)
- `computed_attr_access.lua` → `entity[has_role('x')]` (RR-V0OE)
- `computed_table_key.lua` → `{[1]='x'}`
- `multi_statement.lua` → `true; false` (RR-7VJJ)
- `multi_return_value.lua` → `true, false` (RR-7VJJ)
- `leading_return.lua` → `return false` (RR-8GOP)
- `non_return_stmt.lua` → constructed via direct AST injection if
  possible; otherwise covered by the "single ReturnStmt" check.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **R1: gopher-lua's AST changes shape on upgrade.** Mitigation:
  the walker's default branch rejects unknown node types with a
  named error. CI catches this on the upgrade PR. Severity: low,
  caught early.
- **R2: The `return ` prepend trick stops working** (gopher-lua
  changes return semantics). Mitigation: a smoke test asserts
  `Compile("true")` succeeds. Goes red on any gopher-lua upgrade
  that breaks the trick. Severity: low.
- **R3: Fuzz harness finds a panic in gopher-lua's parser.**
  Mitigation: we wrap `parse.Parse` in `recover()` and convert
  to a `ParseError`. Acceptable — we don't own that parser, so
  the right defence is containment, not fixing upstream.
  Severity: medium if it happens; mitigation is straightforward.
- **R4: Type checker is too strict and rejects rules that should
  work.** Mitigation: ~20 worked examples from the design doc form
  the acceptance test; if any of them fail, the type rules are
  too strict and need relaxing. Severity: low, caught by AC1.
- **R5: Step budget too low for real rules.** Mitigation: the
  worked examples set the floor (none exceed ~50 node visits);
  10k is 200× headroom. Severity: very low.

**Effort:** m (medium). Realistic break-down:

- Day 1: package skeleton, env, value, parse.go (the `return `
  trick), walker accepting use case 1.1 end-to-end. AC1+AC8 green.
- Day 2: complete walker (all allowed node types, type checker),
  Program IR, eval loop, AC1-AC4 green.
- Day 3: step budget, lint helper, fuzz harness, AC5-AC7 green.
- Day 4: edge cases, error message polish, README/doc.go,
  cleanup, send for review.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [ ] User guide / reference docs (defer to ACL integration PR —
      no caller exists yet)
- [ ] CLI help text (no command changes)
- [ ] CLAUDE.md (defer to ACL integration PR — the only rule that
      affects other contributors today is "no Lua on the read
      path," already documented; the predicate package satisfies
      it but doesn't introduce a new rule yet)
- [ ] README.md (no project-level surface change)
- [ ] API docs (godoc on `doc.go` and exported types is enough for
      this PR; no external doc site)
- [x] N/A for user-facing docs in this PR — internal package, no
      consumers yet (RR-S40W)

(User-facing `docs/predicate-language.md` lands with the ACL
integration PR; that's when users gain a way to actually write
rules. Shipping it now would document something that doesn't yet
exist for them.)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **RR-V0OE (critical)** — per-field AST invariants (`AttrGetExpr.Key`
  must be `*StringExpr`). Addressed: walker now enforces per-field
  invariants on each accepted node type; bracket attribute access
  and computed table keys explicitly rejected.
- **RR-VI93 (critical)** — `NumberExpr.Value` is a string; int/float
  discrimination unspecified. Addressed: numeric type model is
  Lua-5.1-style single `Number` (float64-backed); `1`, `1.0`, `1e10`,
  `0xFF`, `1.5e-3` all compile to `Number`. Pinned in AC10.
- **RR-8GOP (significant)** — leading `return` in source. Addressed:
  preprocessor rejects with named error; pinned in AC11.
- **RR-7VJJ (significant)** — multi-statement source. Addressed: AC2
  reject corpus + explicit source-acceptance contract (`len(chunk)==1`,
  single ReturnStmt, `len(Exprs)==1`).
- **RR-XKNO (significant)** — walker stack overflow on deep nesting.
  Addressed: compile-time depth budget (default 256, configurable);
  pinned in AC5 (`TestCompile_RejectsDeeplyNestedExpression`).
- **RR-BUL2 (significant)** — concurrency contract on `*Program`.
  Addressed: invariant documented in `doc.go`; pinned in AC9
  (`TestProgram_Eval_Concurrent` under `-race`).
- **RR-UJW6 (significant)** — `Compile` signature, nil-env handling.
  Addressed: `Compile(env *Env, source string) (*Program, error)`;
  `Eval(b Bindings, opts ...EvalOption) (Value, error)` carries
  per-call options; nil env is a CompileError pinned in AC3.
- **RR-POA2 (significant)** — Lua string + nil-vs-false equality
  semantics. Addressed: equality matrix table in plan + AC10
  (`TestEval_EqualitySemantics`); strings are byte-equal incl. nulls.
- **RR-T4CW (minor)** — five clarity gaps (AC4 inline example,
  public API surface, BOM, `internal/lua` in arch-lint, AC1 corpus
  size). All addressed: worked example added; `Value` is a sealed
  interface; BOM stripped in preprocessor (AC11); `internal/lua` in
  forbidden list (AC8); AC1 pins ≥15 files.
- **RR-S84L (minor)** — `TestCompile_RecoversParserPanics` added in
  AC6 alongside the fuzz harness.
- **RR-S40W (nit)** — Documentation Planning section cleaned up;
  the N/A box reflects the actual state for this PR.
