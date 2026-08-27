---
id: RES-KWWT4J
type: research
title: Computed-property dependency and execution model
summary: Use rela mini interpreter with inferred static dependencies; extend it for value expressions arithmetic and concatenation.
status: done
---

## Problem

Choose a deterministic, safe representation and execution model for entity-local
computed properties written in Lua. The design must support chained computation,
reject cycles at schema load, persist results on every write path, reject
authored values, and avoid turning computation into relation-backed rollups.

## Context

The existing Lua runtime is a hardened gopher-lua sandbox, and `rrule_next`
already implements the motivating recurrence operation. Entity properties are
stored as an untyped map and indexed from store notifications, so computation
only needs to happen before persistence. `internal/statemachine` provides the
compile-once/inject-required precedent. Established computed-field systems also
separate dependency declaration from evaluation: Odoo uses explicit dependency
metadata, while Terraform marks values as provider-computed. This supports an
explicit contract rather than attempting to infer arbitrary program
dependencies.

## Options

### Option 1: Infer dependencies from Lua source

- **Approach**: parse or scan expressions for property reads.
- **Pros**: terse schema and one source of truth.
- **Cons**: lexical scanning is unsound around aliases, dynamic indexing, helper
calls, strings and comments; a full Lua static analyzer is far beyond this
ticket.
- **Effort**: XL and high risk.

### Option 2: Probe evaluation with a recording entity proxy

- **Approach**: execute once at load and record accessed properties.
- **Pros**: no duplicate dependency declaration.
- **Cons**: conditional branches and value-dependent indexing make the observed set
incomplete; load-time dummy values can fail valid expressions; cycles can
escape.
- **Effort**: M, but semantically incorrect.

### Option 3: Extend the existing mini interpreter and infer dependencies

- **Approach**: use `internal/predicate`, rela's strict Lua-expression mini
interpreter. Add a value-expression compile entry point, typed arithmetic and
string concatenation, and expose the statically referenced `entity.<property>`
names from the compiled IR. Compile once, topologically order computed
properties, and reject cycles.
- **Pros**: deterministic and cycle-safe without duplicated metadata; dynamic
indexing, statements, loops, I/O and unknown functions are already rejected;
compile-depth and evaluation-step budgets already exist; typed entity binding
and `today`/`date_add`/`days_between`/`rrule_next` are already implemented.
- **Cons**: this is Lua-compatible expression syntax, not arbitrary full Lua; the
mini interpreter needs contained arithmetic/concatenation extensions.
- **Effort**: L.

## Recommendation

Choose option 3. A property uses a scalar `computed: <expression>` declaration.
The mini interpreter's compiler records every statically accepted
`entity.<property>` read; because computed/dynamic attribute access is already
forbidden, this set is complete and safely drives topological ordering and cycle
detection. No separate `depends_on` declaration is necessary.

Add `CompileValue` alongside the existing boolean-only `Compile` so current
predicate consumers do not change semantics. Add checked typed arithmetic for
Number/Int and Lua `..` concatenation for scalar strings. Evaluation remains a
pure bounded IR walk; there is no VM, timeout, store, HTTP, AI, filesystem,
cache, principal, or mutation binding.

`nil` means remove/unset the computed property. Other evaluation failures abort
the write. Computed properties may be `required` or `unique`; computation runs
before validation and uniqueness checks. A computed property may depend on a
source hidden by read ACL because computation is a trusted write-time
transformation, but its own visibility remains independently configured;
documentation must warn schema authors that making the result more visible can
disclose derived information.

Include the computed block in `PropertyShape` so expression/dependency changes
move the schema hash. Bulk recomputation remains a data-migration follow-up. The
RRULE example belongs in schema documentation and tests, not as a separate
production feature or calendar behavior change.
