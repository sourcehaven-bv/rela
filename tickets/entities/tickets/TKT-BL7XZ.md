---
id: TKT-BL7XZ
type: ticket
title: Client-side condition expression engine (parser + evaluator)
kind: enhancement
priority: medium
effort: m
status: review
---

Build a small, self-contained TypeScript boolean-expression engine for the
data-entry SPA, to be shared by form conditions (wizard
`visible_when`/`required_when`), future ACL button enable/disable, and
view-section visibility.

## Why not reuse `internal/predicate`

`internal/predicate` is Go-only: it consumes gopher-lua's Lua parser, builds a
typed IR, type-checks at compile time, and evaluates with depth/step budgets
(~1,400 non-test LOC). Porting it to JS would require a Lua parser in the SPA
and a second implementation to keep in sync — high cost and drift risk, for a
heavier problem (authorization over the graph) than client-side
condition-checking needs. Instead: a hand-written engine (~300-450 LOC TS). For
this grammar (no statements, no scoping) a Pratt/recursive-descent parser is
~150-250 lines; the evaluator is a small tree-walk.

## Grammar (PINNED — resolves RR-8VZSP)

Precedence is **Lua 5.1**, matching `internal/predicate` (which delegates
precedence to gopher-lua's parser). Low → high binding:

1. `or`
2. `and`
3. comparison: `==` `!=` `<` `<=` `>` `>=` `=~`  (non-associative; chaining like `a < b < c` is a parse error)
4. `not` (unary prefix)
5. primary: literal, dotted reference, function call, `( expr )`

Consequences to pin with tests:
- `a == b and c == d or e`  ⇒ `((a==b) and (c==d)) or e`
- `not a == b`  ⇒ `(not a) == b`  (Lua binds unary `not` **tighter** than comparison — deliberately match; add an AST assertion test)
- `and`/`or` are left-associative.

Literals: single-quoted string `'...'` (with `\'` `\\` escapes), number
(int/float/exp), `true`, `false`, `nil`. References: `form.<field>`,
`entity.<field>`, `current_user.<field>` — dotted, single level; identifier
segments only. Function calls: `name(arg, ...)` — **parsed** but see Host
functions below.

## Not-equal spelling (resolves RR-YTKIC)

Canonical authored/documented spelling is **`!=`** (form authors aren't Lua
users). `~=` is accepted as a silent alias for congruence with predicate. Docs
show `!=` only. NOTE the asterisk on "congruent with predicate": `!=` is not
valid Lua, and our engine is permissive where predicate is strict (below) — so
the grammars are *aligned in surface/precedence*, not identical in semantics.
Document this explicitly.

## Value model, coercion & equality (PINNED — resolves RR-9IQBT)

The engine is **permissive** (filter-style), NOT Lua-strict (predicate makes
cross-type compares a compile error; we don't). Comparison table, applied after
resolving both operands:

- **unset / missing reference** (`form.x` where x absent) resolves to `nil`.
- `== nil` / `!= nil`: true iff the operand is unset/nil.
- **bool literal vs value:** the value is coerced to bool by: JS `true`/`false` as-is; strings `'true'`/`'false'` (case-insensitive) → bool; everything else → compare fails (not equal). So `form.has_processors == true` matches a real boolean `true` OR the string `'true'` (checkbox widgets emit boolean; enum/hand-edited may be string).
- **number literal vs value:** value coerced via `Number(v)`; `NaN` → not equal. So `form.count == 3` matches number `3` or string `'3'`.
- **string literal vs value:** compare `String(v)` byte-for-byte. So `form.kind == 'note'` matches string `'note'`.
- **ordered `< <= > >=`:** attempt numeric coercion of both sides; if both are finite numbers compare numerically, else compare as strings lexicographically. Never throws.
- **`=~` regex:** `new RegExp(literal)` against `String(value)`; **invalid regex → false + `console.warn`, never throws.**

`and`/`or`/`not` operate on the **truthiness** of sub-results (each comparison
yields a real bool; a bare reference in boolean position is truthy iff not
nil/false/empty-string). Unlike predicate (strict bool), a non-bool in boolean
position does NOT error — it coerces. Document the divergence.

## Fail-safe evaluation (PINNED — resolves RR-TNMRC)

Eval errors are **per-node local**, not whole-expression bail: a node that
errors coerces to **false** in place, then evaluation continues. So `brokenRef
or form.ok == true` still yields true if the right side holds. `and`/`or`
**short-circuit** normally (left decides when it can). A parse error is
different: an expression that fails to *parse* evaluates to a constant `false`
at runtime (+ warn), and is caught earlier by the lint (below). Tests must
cover: short-circuit with an erroring left operand on both `and` and `or`.

## Host functions — DEFERRED (resolves RR-P6GVE)

Per decision: **do not build the registry in this ticket.** The only in-scope
consumer (wizard forms) uses `form.<field>` only. The parser DOES accept a
function-call AST node (so the grammar is stable), but eval **rejects any call**
with a local error → false + warn (`no such function: <name>`). The registry +
concrete host functions (`has_role`, `has_relation`) are added by the ACL ticket
when a real caller exists to shape and test the API. This keeps the contract
minimal and unproven surface out of the frozen API.

## Compile/eval split & caching (resolves RR-7VKNB)

Mirror predicate's Program/Eval split: `compile(expr): Program` (parse once →
AST/closure) and `program.eval(bindings): boolean`. Memoize compiled programs by
expression string (a module-level `Map`). `DynamicForm` holds compiled programs
in a computed and re-evals on `formData` change — no re-parse per keystroke.
Volumes are small (~tens of conditions) so this is about avoiding an
obviously-wasteful re-parse loop, not micro-perf.

## Authoring safety / bad-condition surfacing (resolves RR-8GRLD)

Two mechanisms (per decision — dev-console + CLI lint; NOT an in-SPA banner):
1. **Runtime:** on parse/eval failure the engine `console.warn`s with the expression and reason. (Fail-safe → the branch just stays hidden; no crash.)
2. **`rela` CLI config lint (separate small Go surface — lands with the wizard consumer TKT-CHLAJ, since that's when config first carries conditions):** feed each `visible_when`/`required_when` string through the Go **`internal/predicate`** parser (`predicate.Compile` with an env declaring `form`/`entity`/`current_user` records), reporting parse errors and unknown references. This is the shared-grammar payoff — no second Go parser. **Caveat to document:** predicate is *stricter* than the runtime engine (strict bool, cross-type compare = error), so the lint flags a **superset** — it's a "predicate-grammar sanity check", not a 1:1 mirror of runtime behavior. Framed and documented as such. (The lint's Go wiring is tracked with TKT-CHLAJ; this engine ticket only owns the TS runtime + the grammar spec the lint targets.)

## Security

- Property lookup is **prototype-pollution-safe**: reject/skip `__proto__`, `constructor`, `prototype` segments and use own-property access only (mirror `frontend/src/utils/filters.ts` `PROPERTY_NAME_RE`, `^[a-zA-Z_][a-zA-Z0-9_]*$`). Identifier segments only; no computed/bracket access in the grammar (matches predicate's rejection of `entity[expr]`).
- No `eval()`, no `Function()` — hand-written parser over a closed grammar.
- A small **depth cap** on parse (reject absurdly nested expressions) — admin-authored config, but fail-safe.
- Client-side visibility/required is a **UX affordance only, never authorization** — the server re-validates every write regardless.

## Out of scope

- Porting predicate's typed IR / compiler / Lua parser.
- A full JS type-checker.
- The host-function registry (deferred to the ACL ticket).
- Any server-side runtime changes (the CLI lint's Go wiring is tracked with TKT-CHLAJ).

## Acceptance criteria

1. `compile(expr)` parses the pinned grammar (and/or/not, comparisons incl. `!=`/`~=`/`=~`, parens, literals, dotted refs, call nodes) or returns a clear parse error; AST-precedence tests pin the three sample expressions above.
2. `program.eval(bindings)` returns a boolean; `form.`/`entity.`/`current_user.` refs resolve from bindings; compiled programs are memoized by string.
3. The coercion/equality table above holds, with tests for each row (bool/number/string literal vs typed and string values; nil/unset; ordered; regex incl. invalid-regex → false).
4. Fail-safe: per-node eval errors coerce to false locally (short-circuit tests on `and`/`or` with an erroring operand); parse failure → constant false + warn; nothing throws at eval time.
5. Property lookup rejects `__proto__`/`constructor`/`prototype`; unset fields read as nil.
6. Function-call syntax parses but eval rejects any call (`no such function`) — registry deferred.
7. Unit tests cover: every operator, precedence/associativity, parens, short-circuit, nil handling, coercion table, prototype-pollution guard, invalid regex, and the deferred-function path.

## First consumer

TKT-CHLAJ (multi-step wizard forms) uses this engine for
`visible_when`/`required_when` and is the proving ground; it also carries the
`rela` CLI lint wiring. This engine ticket must land first (see `blocks`).
