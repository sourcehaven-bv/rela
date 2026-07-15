---
id: TKT-BL7XZ
type: ticket
title: Client-side condition expression engine (parser + evaluator)
kind: enhancement
priority: medium
effort: m
status: done
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

Consequences pinned with tests:
- `a == b and c == d or e`  ⇒ `((a==b) and (c==d)) or e`
- `not a == b`  ⇒ `(not a) == b`  (Lua binds unary `not` **tighter** than comparison)
- `and`/`or` are left-associative.

Literals: single-quoted string `'...'` (with `\'` `\\` escapes), number
(int/float/exp), `true`, `false`, `nil`. References: `form.<field>`,
`entity.<field>`, `current_user.<field>` — dotted, single level; identifier
segments only. Function calls: `name(arg, ...)` — **parsed** but see Host
functions below.

## Not-equal spelling (resolves RR-YTKIC)

Canonical authored/documented spelling is **`!=`**. `~=` is accepted as a silent
alias for congruence with predicate. Docs show `!=` only. `!=` is not valid Lua
and the engine is permissive where predicate is strict, so "congruent" means
aligned in surface/precedence, not identical semantics.

## Value model, coercion & equality (PINNED — resolves RR-9IQBT)

Permissive (filter-style), NOT Lua-strict. Table applied after resolving both
operands:
- unset/missing reference → `nil`; `== nil`/`!= nil` true iff operand is nil.
- bool literal vs value: real bool as-is; `'true'`/`'false'` (case-insensitive) → bool; else not-equal.
- number literal vs value: **strict decimal** string coercion (DECIMAL_RE) — rejects hex/binary/whitespace/Infinity (RR-ATHC2).
- string literal: byte-for-byte.
- ordered `< <= > >=`: numeric coercion of both; if both finite numbers compare numerically, else lexicographically; never throws.
- non-scalar (array/object) bound values → `nil` (never match a literal) (RR-KR035).

`and`/`or`/`not` operate on truthiness; a non-bool in boolean position coerces
(does not error, unlike predicate).

## Fail-safe EVAL vs throwing PARSE (PINNED — resolves RR-TNMRC, RR-8GRLD; refined after crit review)

The engine has **two responsibilities and one throw boundary**, mirroring the
platform (`new RegExp('[')` / `JSON.parse('{')` throw at construction; only
matching is lenient):

- **`parse(expr): Program` — THROWS `ConditionError`** on a *statically* broken
expression: syntax error, comparison chaining, bare/unknown namespace,
over-budget size (node budget), forbidden field name, or an invalid/oversized
**literal** `=~` pattern. These are config bugs; they fail loud. Memoized by
source (failures cached too, so a repeated bad string re-throws cheaply).
- **`Program.eval(bindings): boolean` — NEVER THROWS.** A per-node
*data-dependent* failure (bad reference, rejected function call, a **dynamic**
`=~ form.pat` pattern only known at eval) coerces to `false` *locally* and
evaluation continues, so one broken leaf doesn't sink an otherwise-valid `or`.
`and`/`or` short-circuit.

**The engine ships NO leniency wrapper and NO one-shot helper.** Deciding what a
broken/unmet condition *means* (hide a branch, surface an inline error, refuse
to render) is the calling layer's policy, made where the error can be surfaced.
A caller that must not throw during render (e.g. a Vue computed) wraps `parse`
in its own try/catch. (This replaced the earlier swallowing `compile()` +
`evaluate()` design, which baked the error-handling decision into the library —
crit review flagged that as taking the decision away from the caller.)

**`=~` asymmetry (deliberate, matches JS):** a *literal* pattern is validated
(syntax + length cap) at parse and throws; a *binding-sourced* pattern is only
knowable at eval and stays fail-safe (false + warn). Documented on the operator.

## Host functions — DEFERRED (resolves RR-P6GVE)

Registry not built here. The parser accepts a function-call AST node (grammar
stable), but eval **rejects any call** → false + warn (`no such function`).
Registry + concrete functions (`has_role`, `has_relation`) land with the ACL
consumer that has a real caller to shape/test the API.

## Complexity / ReDoS bounds (resolves RR-P3HL8, RR-7GDOI, RR-IROUO)

- **Total-node budget (MAX_NODES=500)** enforced per emitted AST node — bounds
flat `and`/`or` chains AND nesting uniformly, so a long chain is rejected at
parse (throws) rather than overflowing the eval-time stack. (Replaced the
nesting-only depth counter that also double-counted parens.)
- **Regex length cap (MAX_REGEX_LENGTH=200)** bounds ReDoS: literal over-length
patterns throw at parse; dynamic ones are rejected fail-safe at eval.

## Authoring safety / bad-condition surfacing (resolves RR-8GRLD)

Because `parse` now **throws**, the calling layer can surface a broken condition
however it wants (inline error, banner, console) — the engine no longer swallows
it. Runtime eval still `console.warn`s on data-dependent failures. The **`rela`
CLI config lint** (Go, reusing `internal/predicate` — a stricter superset; lands
with TKT-CHLAJ) additionally catches parse errors AND unknown-field references
at author time.

## Security

- Prototype-pollution-safe: `__proto__`/`constructor`/`prototype` rejected at parse (throw); own-property lookups only; belt-and-suspenders eval-time guard.
- No `eval()`/`Function()` — hand-written parser over a closed grammar.
- Node budget + regex length cap (above).
- Client-side visibility/required is a **UX affordance only, never authorization** — the server re-validates every write.

## Out of scope

- Porting predicate's typed IR / compiler / Lua parser.
- A full JS type-checker.
- The host-function registry (deferred to the ACL ticket).
- Any server-side runtime changes (the CLI lint's Go wiring is tracked with TKT-CHLAJ).

## Acceptance criteria

1. `parse(expr)` returns a reusable `Program` for the pinned grammar, or **throws `ConditionError`** on any static error (syntax, chaining, bad namespace, over-budget, forbidden field, invalid/oversized literal `=~`); AST-precedence pinned by the sample-expression tests.
2. `program.eval(bindings)` returns a boolean and **never throws**; `form.`/`entity.`/`current_user.` refs resolve; parse results (success + failure) memoized by source.
3. The coercion/equality table holds, with a test per row (bool/number/string vs typed+string, nil/unset, strict-decimal, ordered, non-scalar→nil, literal `=~` match).
4. Throw-vs-fail-safe split: static errors throw at parse; eval-time per-node errors (bad ref, rejected call, dynamic bad/oversized regex) coerce to false locally with short-circuit; nothing throws at eval.
5. Forbidden field names throw at parse; unset fields read as nil; inherited props not resolved.
6. Function-call syntax parses but eval rejects any call (`no such function`) — registry deferred.
7. Unit tests cover every operator, precedence/associativity, parens, short-circuit, nil, coercion table, prototype-pollution, complexity budget, and the literal-vs-dynamic `=~` distinction.

## First consumer

TKT-CHLAJ (multi-step wizard forms) uses this engine for
`visible_when`/`required_when` and is the proving ground; it also carries the
`rela` CLI lint wiring. This engine ticket must land first (see `blocks`).
