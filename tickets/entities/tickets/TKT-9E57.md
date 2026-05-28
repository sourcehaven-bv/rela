---
id: TKT-9E57
type: ticket
title: 'ACL: predicate-backed _fields and _relations resolver (replace stub)'
kind: enhancement
priority: medium
effort: l
status: review
---

## Goal

Replace the dev-only `DemoFieldVerdictResolver` (TKT-G7N5) with a policy-driven
resolver backed by `internal/predicate` (TKT-2QI1). The wire contract
(`_fields`, `_relations`) and SPA renderer stay exactly as shipped — this ticket
changes only the **source of verdicts**.

## Background

- TKT-G7N5 landed the `_fields` / `_relations` wire shape on
per-entity GET, plus the SPA `DynamicForm` / `RelationCards` consumers and a
`FieldVerdictResolver` interface in `internal/dataentry`. The shipped resolver
is a hardcoded `triager-demo` fixture controlled by `RELA_AFFORDANCE_PROFILE`.
- TKT-2QI1 landed `internal/predicate` — a sandboxed Lua-expression
evaluator with a typed Env, `Compile` (parse + walk against allow-list, depth
budget), and `Eval` (per-call step budget, pure-function semantics).

This ticket plugs the two together.

## In scope

### `acl.yaml` extensions

Extend `RoleDef` with three new optional sections. Each grant carries an
optional `when:` *single-string predicate*; absent `when:` unconditionally
grants.

```yaml
roles:
  triager:
    write: [ticket]              # existing per-type write grant
    fields:                       # NEW: per-field write grant
      ticket:
        - field: status
          when: "entity.assignee == current_user.id"
        - field: description
          when: "entity.status != 'done'"
    options:                      # NEW: per-enum-option grant
      ticket:
        - field: status
          option: done
          when: "has_role(current_user, 'closer')"
    relations:                    # NEW: per-relation-type grants
      ticket:
        - relation: implements
          create: true
          remove: false
          when: "entity.status == 'ready'"
        - relation: has-planning
          fields:
            - field: note
              when: "true"
```

Semantics:

- Each grant block is **opt-in**: a field/option/relation with no
matching grant is denied (denied = read-only / hidden / not creatable, depending
on context). This matches `write:`'s closed-world behavior.
- `fields:` controls field **writability** for the type. **Field
visibility** (hide vs read-only) is a separate `visible:` block with the same
shape — kept parallel so the wire shape stays symmetric.
- A single `when:` per grant, compiled at load via
`predicate.Compile`. Compose multiple conditions with `and` / `or` inside the
expression. Compile errors fail policy load (same behavior as today for
unparseable YAML).
- Unknown sub-keys under a grant emit `slog.Warn` per the existing
acl.yaml tolerance convention.

### Resolver implementation

New package `internal/affordances/` (a top-level component, not under
`internal/acl` — arch-lint restricts `acl` to depend only on
`principal`, while the resolver needs `predicate` / `metamodel` /
`entity`):

- `affordances.Resolver` satisfies
`dataentry.FieldVerdictResolver` (compiled programs + role index, looked up per
(principal, type)).
- Predicate env declarations:
  - `entity` — `RecordType` materialised from the metamodel for
the entity's type at policy load. Properties become typed record fields; unknown
properties on the entity surface as nil (consistent with predicate package
semantics).
  - `current_user` — `RecordType{id, type}`.
  - Host funcs: `has_role(user, entity, name)` (entity-scoped,
global ∪ local roles — DR-C6), `has_global_role(user, name)` (DR-C7),
`has_relation(entity, type)`, `count_relations(entity, type)`,
`string_in_list(value, list)`. `is_one_of`/`contains` dropped — can't be typed
under predicate's monomorphic FuncSig (DR-C3).
- Sparse emission: the resolver only writes `false` entries into
the verdict maps. Empty result = no deviations from default.

### Wiring

- `appbuild.New` selects between resolvers:
  1. `RELA_AFFORDANCE_PROFILE=demo` → `DemoFieldVerdictResolver`
(kept for dev / e2e fixtures).
  2. else if `acl.yaml` declares any `fields:`, `options:`, or
`relations:` blocks → policy-backed resolver.
  3. else → `NopFieldVerdictResolver`.
- The policy-backed resolver is `acl.yaml`-discovered at startup;
no separate config knob.

### Audit + wire-parity

- Existing `denied-write` audit op keeps its semantics. The **wire
`rule_id` is unchanged from TKT-G7N5** (`field-affordance:read-only:<field>`,
`field-affordance:enum-filtered:<field>=<opt>`,
`relation-affordance:not-creatable:<type>`, etc.) — introducing a new
`rule_kind=affordance:predicate` prefix would be a wire break for SPA / audit
consumers already parsing the existing shape (DR-C5). Role + predicate
attribution (`role=<role>/grant=<block>.<type>[<i>]`) flows into the audit
record's `Summary` field via a new `AffordanceDenialError.Attribution` channel,
NOT into the external 403 body. Operators get full attribution from the audit
log; clients see only the wire-stable `rule_id`.
- The contract test from TKT-G7N5 (every `_fields[x] == false` ⇒
403 on the corresponding write, every `true` ⇒ 2xx) re-runs against a fixture
`acl.yaml`. Sparse-emission invariant (absent ⇒ default-allowed) re-asserted.

## Out of scope (explicit)

- **Reactive predicates on the SPA.** PATCH already round-trips
`_fields` per response (TKT-G7N5 + the autosave migration in PR #820), so
verdicts refresh whenever an edit lands. Dry-run- on-create (form re-evaluation
as the user types, before any PATCH) is a separate ticket if it proves needed.
- **List-query field-level filtering.** TKT-G7N5 deferred it;
this ticket leaves it deferred. Per-entity GET only.
- **Per-link relation affordances** (different verdicts for
different links of the same type). Needs a wire-shape ticket first.
- **Parameterised verbs** (`transition:done`, `relation:foo:add`)
— TKT-XZEY owns that.
- **Read-side property redaction** (vs hiding via `visible:`).
Same: needs ACL v1 read path first.

## Why this is its own ticket

TKT-G7N5 deliberately shipped with a stub so the wire shape + renderer could be
reviewed and verified end-to-end without entangling them with `acl.yaml` schema
discussion. Plumbing the predicate engine through is a meaningful design surface
in its own right (predicate env shape, sparse emission semantics under
predicates, compile-time vs eval-time error policy, audit rule_id format) and
earns its own design review and code review.

## Dependencies

- TKT-G7N5 (wire shape + stub resolver) — done.
- TKT-2QI1 (predicate language) — done.

## Acceptance criteria

- AC1: `acl.yaml` parses the new `fields:` / `options:` /
`relations:` blocks; predicates compile at load; compile errors surface with
`slog.Error` + non-zero exit (matches existing acl.yaml hard-fail behavior).
- AC2: The resolver emits sparse verdict maps; an `acl.yaml`
with no affordance blocks produces byte-identical wire output to
`NopFieldVerdictResolver`.
- AC3: A field grant with `when:` evaluating false produces
`_fields[name].writable=false` AND a 403 on the corresponding write, with the
TKT-G7N5 wire shape `rule_id=field-affordance:read-only:<field>` (DR-C5). The
denying role + grant path appears in the audit `Summary` only, not the wire
body.
- AC4: An option grant with `when:` false produces
`_fields[name].options[opt]=false` AND a 403 on a write that sets the field to
that option, with `rule_id=field-affordance:enum-filtered:<field>=<opt>`.
- AC5: A relation grant with `create: false` (or `when:` false)
produces `_relations[type].creatable=false` AND a 403 on every relation-create
endpoint (per-relation POST, modern PATCH reconciler, etc. — same chokepoints
TKT-G7N5 gates), with `rule_id=relation-affordance:not-creatable:<type>`.
- AC6: `RELA_AFFORDANCE_PROFILE=demo` still selects the hardcoded
demo fixture, overriding any policy-backed resolver. E2E tests that use the demo
profile keep working.
- AC7: Wire-vs-policy parity test (re-used from TKT-G7N5) passes
against a fixture `acl.yaml`.
