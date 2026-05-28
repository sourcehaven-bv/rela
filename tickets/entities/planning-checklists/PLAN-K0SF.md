---
id: PLAN-K0SF
type: planning-checklist
title: 'Planning: ACL: predicate-backed _fields and _relations resolver (replace stub)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** TKT-G7N5 shipped a hardcoded fixture
(`DemoFieldVerdictResolver`) that proves the wire shape + SPA
renderer end-to-end, but the verdict source is dev-only — every
deployment runs `RELA_AFFORDANCE_PROFILE=none` and gets zero
affordance gates. We need a real policy-driven source so operators
can declare, in `acl.yaml`, which roles may write which fields
under which predicates.

**Scope (IS):**

- Extend `acl.yaml` schema: `RoleDef` gains optional `fields:`,
  `visible:`, `options:`, `relations:` blocks per entity type;
  each grant carries an optional `when:` single-string predicate.
- New package `internal/acl/affordances/` exporting a `Resolver`
  that satisfies `dataentry.FieldVerdictResolver`.
- Compile predicates at policy load via `internal/predicate`
  (TKT-2QI1); compile errors fail load with `slog.Error`.
- Wire into entry points: `cmd/rela-server`, `cmd/rela-desktop`
  select between Demo (env var override), policy-backed
  (acl.yaml has any affordance block), and Nop (default).
- Re-use the existing `denyAffordance` audit chokepoint; expose
  the rule_id format `<role>/<field-or-relation>` via the
  resolver's verdict metadata.
- Re-use the wire-parity contract test from TKT-G7N5 against a
  fixture `acl.yaml`.

**Scope (IS NOT):**

- Reactive predicates in the SPA (separate ticket if needed —
  PATCH already round-trips `_fields` per response).
- List-query field-level filtering (deferred by TKT-G7N5).
- Per-link relation affordances (needs wire-shape change).
- Parameterised verbs `transition:done` / `relation:foo:add`
  (TKT-XZEY).
- Read-side property redaction (needs ACL v1 read path).
- Group expansion, transitive role-relations (ACL v1 territory).
- Inherited local roles via `inherit_roles_through` (depth-capped
  graph walk). v1 resolves DIRECT local roles only — a role
  conferred by a `role_relations` edge straight from the principal
  to the evaluated entity. Inheritance is ACL-v1 (DR-C6).

**Acceptance Criteria:** (re-stated from ticket, with concrete test mapping below)

1. AC1 — acl.yaml parses new blocks; predicates compile at
   load; compile errors fail with `slog.Error`.
2. AC2 — sparse emission; absent affordance blocks = byte-identical
   wire to Nop.
3. AC3 — field `when:` false ⇒ `writable=false` AND 403 on write
   with `rule_kind=affordance:predicate`, `rule_id=<role>/<field>`.
4. AC4 — option `when:` false ⇒ `options[opt]=false` AND 403 on
   write setting that option.
5. AC5 — relation `create:false`/`when:` false ⇒
   `creatable=false` AND 403 on every relation-create chokepoint.
6. AC6 — `RELA_AFFORDANCE_PROFILE=demo` still selects
   `DemoFieldVerdictResolver`; e2e demo tests keep working.
7. AC7 — wire-parity contract test from TKT-G7N5 passes against
   fixture acl.yaml.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **`internal/predicate` (TKT-2QI1)** — done. Public API:
  `predicate.NewEnv()`, `Env.DeclareVar(name, Type)`,
  `Env.DeclareFunc(name, FuncSig)`, `predicate.Compile(env,
  source) (*Program, error)`, `Program.Eval(ctx, *Bindings)
  (Value, error)`. Programs are immutable + safe for concurrent
  Eval; Bindings built per-call. Numeric model = float64;
  RecordType for entity, primitive types for scalars.
  `internal/predicate/doc.go:1` and `env.go:11-180`.
- **`internal/acl.LoadPolicy`** (`internal/acl/policy.go:89`) —
  tolerant YAML loader: warn-and-continue on unknown top-level
  keys; hard error on unparseable YAML. `knownPolicyKeys` map
  drives the warnings (`policy.go:71`). Pattern to follow for
  unknown sub-keys under `fields:` / `options:` / `relations:`.
- **`acl.Declarative`** (`internal/acl/declarative.go:30`) — the
  existing policy-driven ACL implementation. Pattern: holds
  `*Policy`, evaluates per-request against the principal carried
  on ctx, returns structured `Decision`. The new affordance
  resolver mirrors this pattern: hold compiled programs + role
  index, evaluate per (principal, entity).
- **`dataentry.FieldVerdictResolver`** interface
  (`internal/dataentry/affordances.go:93`) — the contract we
  satisfy. Returns `FieldVerdicts{Writable, Visible, Options}`
  and `RelationVerdicts{Types}`. Both sparse — only deviations
  populated. **Critical:** absence of a `Writable[name]` entry
  means default-allowed; the policy must explicitly set `false`
  to deny.
- **Existing wire-vs-policy contract test** in
  `internal/dataentry/affordances_test.go` (`affordances_contract_test.go`
  per CLAUDE.md). Re-use against a fixture `acl.yaml`.
- **`App.denyAffordance`** (`affordances.go:365`) — the
  chokepoint that writes 403 + emits `denied-write` audit. Used
  from six call sites in `api_v1.go`. The current rule_id is
  derived from `AffordanceDenialError.RuleID()` =
  `<rule>:<path>` where `rule` is one of
  `field-affordance:hidden`, `:read-only`, `:enum-filtered`,
  etc. **Decision below:** keep this wire format; the policy
  source surfaces as additional context in `Reason` and an
  optional second segment in `Path`, not as a new rule prefix.

**Why not a different evaluator (CEL, expr, EDN)?** TKT-2QI1
already settled this — predicate is in the binary, type-checked,
sandboxed, with familiar Lua-expression syntax. No re-litigation.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### 1. `acl.yaml` schema extension

Extend `acl.RoleDef` in `internal/acl/policy.go`:

```go
type RoleDef struct {
    Write       []string `yaml:"write"`
    Read        []string `yaml:"read"`
    Permissions []string `yaml:"permissions"`
    // NEW: affordance grants, all optional and opt-in.
    Fields    map[string][]FieldGrant    `yaml:"fields"`
    Visible   map[string][]FieldGrant    `yaml:"visible"`
    Options   map[string][]OptionGrant   `yaml:"options"`
    Relations map[string][]RelationGrant `yaml:"relations"`
}

type FieldGrant struct {
    Field string `yaml:"field"`
    When  string `yaml:"when,omitempty"`
}

type OptionGrant struct {
    Field  string `yaml:"field"`
    Option string `yaml:"option"`
    When   string `yaml:"when,omitempty"`
}

type RelationGrant struct {
    Relation string             `yaml:"relation"`
    Create   *bool              `yaml:"create,omitempty"`
    Remove   *bool              `yaml:"remove,omitempty"`
    Fields   []RelationMetaGrant `yaml:"fields,omitempty"`
    When     string             `yaml:"when,omitempty"`
}

type RelationMetaGrant struct {
    Field string `yaml:"field"`
    When  string `yaml:"when,omitempty"`
}
```

Pointer-to-bool for `Create`/`Remove` distinguishes "unset" (use
default — allow if there's a grant at all) from "explicit
false". Default when only `When` is set: both `create` and
`remove` are true (grant exists → operations allowed if the
predicate passes).

### 2. New package `internal/acl/affordances/`

Files:

- `resolver.go` — `Resolver` struct + `New(policy *acl.Policy, meta *metamodel.Metamodel) (*Resolver, error)`. Compiles every `when:` predicate at construction time (collects all errors, returns the first wrapped with full path: `roles.triager.fields.ticket[0].when: <err>`). Builds an index of `(roleName, entityType) → []compiledFieldGrant` etc.
- `env.go` — Per-entity-type `*predicate.Env` builder: declares `entity` as `RecordType` over the metamodel's properties for that type (string for string/enum/date/rrule, number for integer, bool for boolean — list types map to `ListType{Elem}`), `current_user` as `RecordType{id: string, type: string}`, and host funcs `has_role` (3-arg, entity-scoped — DR-C6), `has_global_role` (DR-C7), `has_relation`, `count_relations`, `string_in_list` with their signatures.
- `bindings.go` — Per-call binding builder: snapshots the entity's properties into a `predicate.Record`, snapshots the principal into `current_user`, and wires the host funcs against the actual store/graph snapshot.
- `resolver_test.go` — Unit tests for the verdict-computation path: each grant kind, sparse emission, `when:` evaluation.
- `acl_yaml_test.go` — Round-trip tests: parse a fixture acl.yaml, build resolver, assert verdicts on synthetic entities.

`Resolver.FieldVerdicts(ctx, e)` and `Resolver.RelationVerdicts(ctx, e)`:

1. Look up the principal from ctx via `principal.From`.
2. Compute the `(principal, e)` effective role set (DR-C6):
   global roles (`Assignments[user]` ∪ `"default"`, the existing
   `Declarative.effectiveRoles` logic) PLUS direct local roles —
   for each `role_relations` type R with `Confers: X`, if the
   snapshot graph has `user --R--> e`, add role X. Resolved once
   per call against the snapshot (CLAUDE.md snapshot-once;
   acl-design.md §"Snapshot semantics" warns this is Plone's #1
   stale-cache bug class). Inherited local roles deferred.
3. For each role in that set, evaluate its grants on `e.Type`
   under the per-role-per-type opt-in model (DR-S3). The
   cross-role semantic is **union**: if ANY role grants write on
   field X with a passing `when`, X is writable.
4. Build the sparse `FieldVerdicts` / `RelationVerdicts`: emit
   `false` for any field/option/relation that has no granting
   role under a passing predicate.

**Critical: closed-world semantics.** The current `Nop` resolver
returns empty maps = "everything default-allowed." The policy
resolver must instead emit `false` for everything **not granted**
— `fields:` is opt-in like `write:`. But: only when the role has
*any* `fields:` declaration for that type. A role with `write:
[ticket]` and no `fields:` block at all should leave fields fully
writable (else this becomes a backwards-incompatible change for
every existing acl.yaml).

**Decision:** "opt-in per type" — if a role declares `fields:` for
type T at all, that role's per-field grants are closed-world for
T (unlisted fields denied). If no role declares any `fields:` for
T, all fields default-writable for that type. Same logic for
`visible:`, `options:`, `relations:`.

### 3. Wire-up

- `cmd/rela-server/main.go:119` and
  `cmd/rela-desktop/main.go:157` currently pass
  `dataentry.ResolverFromProfile(os.Getenv("RELA_AFFORDANCE_PROFILE"))`.
- Change `ResolverFromProfile` signature:
  `ResolverFromProfile(profile string, policy *acl.Policy, meta *metamodel.Metamodel) FieldVerdictResolver`.
- Logic:
  1. `profile == "demo"` → `DemoFieldVerdictResolver{}` (override).
  2. `policy != nil && policyHasAffordances(policy)` →
     `affordances.New(policy, meta)`.
  3. else → `NopFieldVerdictResolver{}`.
- Both entry points already have `svc.ACL()` and the metamodel
  via `svc.Meta()`; expose `svc.ACLPolicy()` to surface the raw
  `*Policy` (currently held internally by `appbuild.Services`).
- The resolver constructor's compile errors propagate up — same
  fail-fast behavior as `LoadPolicy` for malformed YAML.

### 4. Audit + denial path

- `denyAffordance` (`affordances.go:365`) is unchanged.
- The resolver's verdict carries no extra metadata — it just
  returns `Writable[field]=false`. The downstream
  `validateFieldWrite` produces `AffordanceDenialError{Rule:
  RuleFieldReadOnly, Path: field}`, which renders as
  `rule_id=field-affordance:read-only:<field>`.
- Predicate-evaluation errors at runtime (e.g., a host func
  errors): the resolver logs `slog.Warn` and treats the grant
  as not-applied (deny-by-default — fail closed). The verdict
  for the affected field surfaces as `writable=false`.
- The `rule_kind=affordance:predicate` / `rule_id=<role>/<field>`
  format from the ticket's AC3 is **dropped** in favor of the
  existing wire shape. Reason: introducing a new rule_kind
  prefix is a wire change; the existing format is what TKT-G7N5
  callers (SPA, audit consumers) already parse. The role/predicate
  attribution lives in the audit row's `Summary` field and in
  server-side debug logs, not on the wire.
- AC3/AC4/AC5 are amended accordingly during implementation:
  the wire assertion is `rule_id=field-affordance:read-only:<field>`,
  not a new prefix.

### 5. Predicate env shape (AMENDED per DR-C2, DR-C3)

```go
// Declared once per entity type, cached on the resolver.
env := predicate.NewEnv()
env.DeclareVar("entity", recordTypeFor(meta, entityType))
env.DeclareVar("current_user", predicate.RecordType{
    "id":   predicate.StringType,
    "type": predicate.StringType,
})
// has_role takes BOTH current_user AND the target entity (DR-C6):
// local roles (owner, editor) are entity-scoped, conferred by a
// role-relation like alice --owner-of--> TKT-001. "is current_user
// an owner?" is meaningless without "owner of WHAT" — the entity
// supplies that scope.
env.DeclareFunc("has_role", predicate.FuncSig{
    Params: []predicate.Type{predicate.RecordType{}, predicate.RecordType{}, predicate.StringType},
    Return: predicate.BoolType,
})
// has_global_role is the entity-INDEPENDENT check (DR-C7): for
// global roles assigned via `assignments` (admin, triager, etc.)
// where no entity scope applies. Two clearly-typed funcs beats one
// func with a nullable entity arg whose semantics shift — the
// nil-entity footgun the design review warned about. In THIS
// ticket every predicate has an entity (field/relation affordances
// are per-entity), so has_global_role is a convenience that lets a
// predicate skip the entity arg when it only cares about a global
// role. Collection-scope predicates (gating `create` before any
// entity exists) are a future ticket; when they land, they declare
// an env WITHOUT `entity` and only has_global_role is available —
// no nil entity is ever constructed.
env.DeclareFunc("has_global_role", predicate.FuncSig{
    Params: []predicate.Type{predicate.RecordType{}, predicate.StringType},
    Return: predicate.BoolType,
})
env.DeclareFunc("has_relation", predicate.FuncSig{
    Params: []predicate.Type{predicate.RecordType{}, predicate.StringType},
    Return: predicate.BoolType,
})
env.DeclareFunc("count_relations", predicate.FuncSig{
    Params: []predicate.Type{predicate.RecordType{}, predicate.StringType},
    Return: predicate.NumberType,
})
env.DeclareFunc("string_in_list", predicate.FuncSig{
    Params: []predicate.Type{predicate.StringType, predicate.ListType{Elem: predicate.StringType}},
    Return: predicate.BoolType,
})
```

**Metamodel → predicate type mapping (NO `AnyType` — see DR-C2):**

| metamodel | predicate type |
|-----------|----------------|
| string, enum, date, rrule | `StringType` |
| integer | `NumberType` |
| boolean | `BoolType` |
| `list: true` of string-ish | `ListType{Elem: StringType}` |
| `list: true` of integer | `ListType{Elem: NumberType}` |
| any unsupported type | **NOT declared in env** — predicate referencing it fails at compile with a clear operator-facing error (`predicate: env: unknown variable "entity.<prop>"`) |

The "not declared" choice is deliberate: a predicate referencing a
property the env doesn't know about should be a hard operator
error at server startup, not a silent runtime quirk.

### 6. Host function implementations (AMENDED per DR-C3)

Bound per-`Resolver.FieldVerdicts` call against a single store
snapshot taken at the top of the resolver call (DR-S5; consistent
with CLAUDE.md "capture state once per operation"):

- `has_role(user_record, entity_record, role_name string) bool` —
  checks whether the user holds `role_name` **scoped to the given
  entity** (DR-C6). The effective role set for `(user, entity)` is
  global roles (from `Assignments`) ∪ local roles conferred on
  `entity` by a `role_relations` edge from the user (e.g.
  `alice --owner-of--> entity` confers `owner` per
  `RoleRelationDef.Confers`). The `.ignored/acl-design.md`
  four-layer model (users → groups → roles → local roles) requires
  this entity dimension; a global-only `has_role` could never
  express owner/editor/reviewer affordances.
- `has_global_role(user_record, role_name string) bool` — the
  entity-INDEPENDENT check (DR-C7) for global roles assigned via
  `assignments` (admin, triager, etc.). Use when a predicate only
  cares about a global role and doesn't want to thread the entity.
  Distinct func rather than a nullable entity arg on `has_role` —
  avoids the nil-entity footgun. (`has_role(u, e, r)` is the
  superset: it ALSO matches global roles, since the effective set
  is global ∪ local. `has_global_role` is the convenience form
  that ignores local roles by construction.)
- `has_relation(entity_record, rel_type string) bool` — checks
  whether the entity has any outgoing relation of the given type.
- `count_relations(entity_record, rel_type string) number` — count
  of outgoing edges of the given type.
- `string_in_list(value string, allowed list_of_string) bool` —
  typed membership.

**Effective-role resolution (DR-C6).** The resolver computes the
`(principal, entity)` effective role set once per
`Resolver.FieldVerdicts` call against the snapshot:

1. Global roles: `Assignments[user]` ∪ `default` (existing
   `Declarative.effectiveRoles` logic).
2. Local roles: for each `role_relations` type R with
   `Confers: X`, if the graph has `user --R--> entity`, add role
    X. (v1 = direct local roles only; inherited local roles via
   `inherit_roles_through` are ACL-v1 / deferred — flagged below.)
   `has_global_role` (DR-C7) consults only step 1 of this set.

This set is what `has_role(user, entity, name)` consults. It is
also what gates which roles' `fields:`/`options:`/`relations:`
grants apply for this entity — local-role grants (a role granted
only via `owner-of`) participate in the same per-role-per-type
opt-in union (DR-S3). **NOTE:** this means a grant block keyed on
a local role (e.g. `roles.owner.fields.ticket`) only contributes
for entities where the principal holds that local role — naturally
entity-scoped, which is the whole point of local roles.

Dropped from v1 scope (DR-C3): `is_one_of`, `contains` — neither
can be typed safely under predicate's monomorphic FuncSig without
falling through to `AnyType`-in-Params, which defeats the
compile-time type checker for policy authors. Polymorphic variants
land in follow-up tickets if predicate authors demonstrate the
need. Numeric membership: `x == 1 or x == 2 or x == 3` (verbose
but type-checked).

Implementations live in `internal/acl/affordances/` (not in
`internal/predicate` — predicate is host-agnostic).

**Alternatives considered:**

- **`internal/acl/affordances` vs `internal/acl/declarative_affordances.go`**: sub-package wins. Affordances have their own YAML schema, their own predicate env, their own tests — fits the rela "separate package per subsystem" pattern (CLAUDE.md). The sub-package also keeps `acl.Declarative` un-bloated.
- **Single string `when:` vs list of conditions**: single string. Composing with `and`/`or` inside the expression is the same in predicate as it would be in YAML. Lists add YAML rope without expressive power. User confirmed.
- **Per-grant `when:` vs role-level `when:`**: per-grant. A role like "triager" gating `status` on `entity.assignee == current_user.id` AND gating `priority` on a different predicate is the canonical case.
- **New `rule_kind=affordance:predicate` wire format**: rejected. Wire change, and the SPA already routes the existing `field-affordance:read-only:<field>` to the right UI affordance. The role/predicate attribution belongs in audit + server logs, not on the wire.

**Files to modify:**

- `internal/acl/policy.go` — extend `RoleDef`; add new grant types; update `knownPolicyKeys` (still tolerant for unknown grants).
- `internal/acl/policy_test.go` — parse-roundtrip tests for the new schema.
- `internal/acl/affordances/resolver.go` — **new**.
- `internal/acl/affordances/env.go` — **new**.
- `internal/acl/affordances/bindings.go` — **new**.
- `internal/acl/affordances/resolver_test.go` — **new**.
- `internal/acl/affordances/acl_yaml_test.go` — **new**.
- `internal/dataentry/affordances_stub.go` — update `ResolverFromProfile` signature and dispatch logic.
- `internal/dataentry/affordances_test.go` — re-run the wire-parity contract test against a fixture acl.yaml resolver.
- `cmd/rela-server/main.go` — pass policy + meta into ResolverFromProfile.
- `cmd/rela-desktop/main.go` — same.
- `internal/appbuild/appbuild.go` — expose `Services.ACLPolicy() *acl.Policy` (currently only the ACL is exposed).
- `internal/appbuild/testfixture.go` — same change for tests.
- `docs/security.md` — document the new acl.yaml fields with examples.
- `docs/data-entry/api-reference.md` — note that affordances are now policy-driven, link to security.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Validation | On invalid |
|--------|------------|------------|
| `acl.yaml` (operator-supplied at server start) | YAML parse + `predicate.Compile` on every `when:` | Hard fail at startup (matches existing `LoadPolicy` behavior for parse errors). Unknown sub-keys: `slog.Warn` + ignore (matches existing tolerance convention). |
| Per-request principal (ctx) | Existing `principal.From` middleware | Already validated upstream — same trust assumption as `acl.Declarative`. |
| Per-request entity (loaded from store) | Already validated upstream by store layer | Entity properties may be of unexpected runtime types (storage is permissive — CLAUDE.md "tolerate temporarily invalid data"). The binding code coerces to `predicate.Value` defensively: unconvertible values bind as `Nil`, not an error. |

**Security-Sensitive Operations:**

1. **Predicate evaluation as authorization signal.** Same trust
   posture as `Declarative.AuthorizeWrite` — the policy is the
   source of truth, the operator owns the policy file.
   `predicate` is sandboxed (no I/O, no goroutines, step budget,
   depth budget). Host funcs are explicitly named; the resolver
   does not register any arbitrary-code-execution capability.

2. **Fail-closed on predicate errors.** A runtime error in a
   `when:` predicate (e.g., a host func panics or returns
   error) MUST NOT default to allow. The resolver logs and
   treats the grant as not-applied. **Verified by test.**

3. **Cross-role grant union — deny-by-default is preserved
   per-type.** If role A grants `fields.ticket.status` and role
   B grants `fields.ticket.description`, a user with both roles
   gets writability of BOTH. But a user with neither role and
   no other grant gets nothing — closed-world.

4. **No predicate evaluation on the read path beyond verdict
   computation.** Per CLAUDE.md "Don't run user-supplied Lua on
   the read path" — predicate IS NOT user-supplied; it's
   operator-supplied via acl.yaml, same trust class as the
   policy file itself. But: per-entity GET evaluates predicates
   to populate `_fields` / `_relations`. The cost is bounded by
   the step budget (10k per Eval) and the number of grants
   (operator-controlled — practical sizes are dozens). List
   queries do NOT evaluate predicates (out of scope) — this
   keeps the perf cliff at bay per the ACL design rationale.

5. **Predicate environment cannot reach the store directly.** Host
   funcs are the only escape; each is bounded and named. No
   `os.ReadFile` reachable, no `entitymanager.CreateEntity`
   reachable.

**Error handling — no information leaks:**

- 403 body uses the existing wire shape: `{error, rule_kind,
  rule_id, reason}`. The reason is the field name + "is not
  writable" — same as TKT-G7N5; no policy details leak (role
  name, predicate text).
- Server-side `slog.Debug` carries the policy attribution
  (`role=triager grant=fields.ticket.status when="..."`) for
  operators debugging deny decisions; not exposed via API.
- Audit `Summary` includes `rule_kind=affordance rule_id=...`
  but not the predicate source (the predicate text is in
  acl.yaml, which audit readers should consult separately).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|----|------|
| AC1 | `policy_test.go`: round-trip parse of fixture acl.yaml with `fields:`, `visible:`, `options:`, `relations:` blocks. `resolver_test.go`: `affordances.New` with malformed `when:` returns wrapped error including grant path. |
| AC2 | `affordances_contract_test.go` (existing): re-run with a `Nop`-equivalent acl.yaml (only `write:`, no affordance blocks). Assert byte-identical wire to the Nop resolver. |
| AC3 | `resolver_test.go`: build resolver from fixture acl.yaml with `fields.ticket[0].when: "entity.status == 'ready'"`. Synth entity with `status=done`. Assert `FieldVerdicts.Writable["status"] == false`. End-to-end: `api_v1_test.go` PATCH to that field returns 403 with `rule_id=field-affordance:read-only:status`. |
| AC4 | `resolver_test.go`: option grant with falsy `when:`. Assert `FieldVerdicts.Options["status"]["done"] == false`. E2E: PATCH setting `status=done` returns 403 with `rule_id=field-affordance:enum-filtered:status=done`. |
| AC5 | `resolver_test.go`: relation grant with `create: false`. Assert `RelationVerdicts.Types["implements"].Creatable == false`. E2E: POST to each relation-create endpoint returns 403 with `rule_id=relation-affordance:not-creatable:implements`. Same for modern PATCH reconciler. |
| AC6 | `affordances_stub_test.go` (existing): `ResolverFromProfile("demo", nil, nil)` returns `DemoFieldVerdictResolver{}`. With a non-nil policy that has affordance blocks AND profile=demo, demo still wins. |
| AC7 | The existing wire-parity contract test re-runs with a fixture acl.yaml resolver and passes byte-identical to TKT-G7N5's expectations. |

**Edge Cases:**

- **Empty `when:`**: grants unconditionally. Tested.
- **`when:` referencing unknown variable**: compile error at
  load. Tested.
- **`when:` referencing entity property not in metamodel**:
  compile error (predicate type-checks against the declared
  RecordType). Tested.
- **`when:` returning non-bool** (e.g., `entity.title`): compile
  error (predicate enforces return type at the relational/logical
  combinator level; a bare expression returning a string is
  rejected by the resolver wrapper with an explicit error: "when:
  must evaluate to a boolean").
- **Grant declared for unknown role**: ignored with `slog.Warn`,
  matches existing `Declarative` behavior for unknown role names
  in `Assignments`.
- **Grant declared for unknown entity type**: ignored with
  `slog.Warn`. Same rationale.
- **Field grant for unknown field in known type**: ignored with
  `slog.Warn`. Operators iterate; typo shouldn't brick.
- **Principal with no roles** (anonymous): only `default` role
  applies if present; otherwise zero grants. Closed-world ⇒
  every opt-in field is denied. Matches existing
  `Declarative.AuthorizeWrite` deny-by-default.
- **`Create: nil, Remove: nil` on RelationGrant**: defaults both to true (operator opted in via the grant existing at all).
- **`Create: ptr(false), Remove: nil`**: create denied, remove allowed.
- **Multiple roles, conflicting grants**: union semantics — any
  passing grant allows. Same as `write:`.
- **Predicate runtime error during per-entity GET**: fail-closed
  on that grant; verdict surfaces as deny; `slog.Warn` for
  operator visibility. Does NOT 500 the GET.
- **Predicate step-budget exhaustion**: same — fail-closed.
- **Resolver called with nil entity**: returns zero verdicts
  (existing pattern in `validateFieldWrite`).
- **acl.yaml present but no affordance blocks**: behavior
  byte-identical to Nop (AC2). No closed-world for any type.
- **acl.yaml present with affordance blocks for type A but not
  type B**: type B is permissive; type A is closed-world.
  Per-type opt-in.

**Negative Tests:**

- Compile error in `when:` (parse error, unknown var, type error)
  ⇒ resolver constructor returns error with the grant path in the
  error message, server exits non-zero at startup.
- Host func error at runtime ⇒ verdict deny, `slog.Warn`, no HTTP
  500 response.
- Stale SPA POSTing a hidden field ⇒ 403 with hidden-shape rule
  (per the F8 closure that TKT-G7N5 added).
- Policy load failure ⇒ existing `LoadPolicy` hard-fail, no change.

**Integration tests:**

- `cmd/rela-server/main_acl_test.go` (existing) — extend with a
  fixture acl.yaml carrying affordance blocks; assert the GET
  response carries the expected `_fields` / `_relations` shape
  AND that PATCH against denied fields returns 403 with the
  right rule_id.
- `internal/dataentry/api_v1_test.go` — re-run a subset of the
  TKT-G7N5 contract tests against the policy resolver to prove
  wire-parity.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|------------|
| Performance: per-entity GET evaluates N predicates × M roles. | Per CLAUDE.md, list queries skip predicates entirely (out of scope). Per-entity GET is bounded: N≈dozens of grants, M=user's effective roles. Step budget caps any single Eval at 10k. No caching in v1 — measure first, cache later if needed. |
| Closed-world semantics break existing deployments. | Per-type opt-in: a role with no `fields:` block leaves fields permissive for that type. Existing acl.yaml files keep working unchanged. **AC2 pins this.** |
| Predicate env divergence from store types. | Type mapping table is explicit; unknown property types map to `AnyType` with a debug log; tests cover every metamodel primitive type. |
| Resolver+policy lifecycle (policy reload). | v1: no live policy reload. Resolver is built once at startup. Matches existing `Declarative` lifecycle. |
| Rule_id wire format drift between Demo and policy resolvers. | Both go through the same `validateFieldWrite` / `validateRelationOp` → `AffordanceDenialError` path. Wire format is determined by the validators, not the resolver. **Contract test pins this.** |
| Per-type `RecordType` mutation during reload. | `predicate.Env` is mutable until first Compile. Build the env, compile every program, then discard the env — programs hold what they need. Per-type envs are not shared across types. |

**Effort:** L (matches the ticket). Bulk of the work is the new
sub-package + tests; the YAML schema extension and the wire-up
changes are mechanical.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `docs/security.md` gets a new
  "Affordances" section under the ACL reference. Schema +
  examples for `fields:`, `visible:`, `options:`, `relations:`.
- [ ] CLI help text — N/A
- [ ] CLAUDE.md — possibly a small note under "Authorization
  (`internal/acl`)" referencing the new sub-package; decide
  during implementation.
- [ ] README.md — N/A
- [x] API docs — `docs/data-entry/api-reference.md` note that
  affordances are now policy-driven; link to security.md for
  the schema.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan
- [x] Crit review approved (3 rounds; DR-C6, DR-C7 added + addressed)

**Design Review Findings (addressed inline below; tracked as RRs against TKT-9E57):**

### Critical (all addressed in the amended approach)

- **DR-C1: `Visible` verdict semantics under closed-world.** The
  original draft listed `visible:` as a YAML block sibling but never
  spelled out the resolver logic, and the closed-world rule means an
  opt-in `visible:` block invisibly hides every undeclared field —
  including stripping them from wire `properties` and from `_title`.
  **Fix applied:** `visible:` follows the same per-type opt-in rule
  as `fields:` (a role declaring `visible:` for type T is closed-
  world for T's visibility; absent block = fully visible). Hidden
  takes precedence over read-only when both apply (matches
  `affordances.go:585`); a hidden field is skipped from `_fields`
  entirely. Added explicit ACs and test scenarios below.

- **DR-C2: `AnyType` would fail every Eval, not bypass type
  checking.** `predicate/eval.go:79-100`'s `runtimeTypeAccepts` only
  short-circuits for `RecordType` and `ListType`; `AnyType`
  (`primitiveType{"any"}`) falls through to a name-match equality
  that rejects every concrete value. The original draft proposed
  `AnyType` for unknown metamodel property types, which would have
  promoted a soft data-integrity issue (permissive storage carrying
  off-type values) into a hard deny under fail-closed — locking
  operators out of editing entities with drifted data.
  **Fix applied:** dropped `AnyType` entirely from the entity
  RecordType. Defined a value-coercion contract at the binding
  layer:

  | metamodel | runtime stored | binds as |
  |-----------|----------------|----------|
  | string / enum / date / rrule | string | `String(v)` |
  | string / enum / date / rrule | other or missing | `Nil` |
  | integer | int / float64 / numeric string | `Number(v)` |
  | integer | other or missing | `Nil` |
  | boolean | bool | `Bool(v)` |
  | boolean | "true" / "false" | `Bool(v)` |
  | boolean | other or missing | `Nil` |
  | list-typed | `[]interface{}` of elem | `List(...)` |
  | list-typed | scalar | `List([elem])` (single-elem promotion) |
  | list-typed | other or missing | `List([])` |
  | unsupported metamodel type | any | NOT declared in env (predicate referencing it fails to compile — operator-facing error) |

  Coercion is best-effort: a stored map where a string is declared
  binds as `Nil`, not an Eval error. Authors writing `entity.foo ==
  "x" and entity.foo ~= nil` (or just `entity.foo == "x"` — nil ==
  "x" is `false`) handle the missing-property case cleanly. Failing
  closed on coercion was rejected: it turns hand-edit data drift
  into permissions outages.

- **DR-C3: `is_one_of` and `contains` cannot be declared.**
  `predicate.FuncSig.Params` is `[]Type` with no union/generic.
  `is_one_of(value, list)` requires polymorphism the type system
  doesn't have. The escape hatch (`AnyType` in Params + runtime
  type-switch) defeats the compile-time type checker for policy
  authors.
  **Fix applied:** drop `is_one_of` and `contains` from this
  ticket's host-func set. Ship the typed primitive
  `string_in_list(value string, allowed list_of_string) bool` as
  the only collection primitive. Predicate authors needing numeric
  membership write `x == 1 or x == 2 or x == 3` — verbose but
  unambiguous and type-checked. The full host-func set for v1:

  | Function | Sig | Use |
  |----------|-----|-----|
  | `has_role(user, entity, name)` | `(Record, Record, String) → Bool` | role membership scoped to entity (global ∪ local roles — DR-C6) |
  | `has_global_role(user, name)` | `(Record, String) → Bool` | global-role-only check, entity-independent (DR-C7) |
  | `has_relation(entity, type)` | `(Record, String) → Bool` | any outgoing edge of type |
  | `count_relations(entity, type)` | `(Record, String) → Number` | count of outgoing edges |
  | `string_in_list(value, allowed)` | `(String, List<String>) → Bool` | membership |

  Other helpers (numeric_in_list, contains_substring) can land in
  follow-up tickets if predicate authors demonstrate the need.

- **DR-C4: Empty-list grant semantics ambiguous.** YAML
  `fields: {ticket: []}`, `fields: {ticket: null}`, and `fields:
  {ticket:}` parse differently and the plan didn't say which the
  resolver distinguishes.
  **Fix applied (corrected during implementation):** the opt-in
  signal is the **presence of the per-type key** in the map.
  Verified yaml.v3 behavior (`TestLoadPolicy_AffordanceGrants_
  OptInIsKeyPresence`):
  - `fields: {ticket: []}` → present key, non-nil empty slice.
  - `fields: {ticket:}` (null) → present key, nil slice.
  - `fields:` key absent → key NOT in map.

  Both empty-list and null forms yield a **present** key, so both
  are opt-in (closed-world deny-all when zero grants). Only an
  absent key is permissive. The original plan claimed null/empty
  decoded to "absent" — that was wrong; yaml.v3 keeps the key for
  a null value. Hanging the security decision on nil-vs-empty-slice
  would be too subtle, so the contract is simply: present (either
  form) = opt-in. Tests pin all three shapes.

- **DR-C5: Wire-format amendment changed ACs without updating the
  ticket, and the dropped role/predicate attribution hurts
  operator debuggability.**
  **Fix applied, two parts:**
  1. The ticket text (TKT-9E57) will be updated to match before
     PR — task added to the implementation checklist. Wire-format
     stays as TKT-G7N5 shipped:
     `rule_id=field-affordance:read-only:<field>` etc.
  2. Operator attribution is preserved via a **two-channel split**:
     the wire response is unchanged (no role/predicate in the
     external 403), but `AffordanceDenialError` gains an
     `Attribution string` field (e.g.,
     `role=triager/grant=fields.ticket[0]/when=...`) that flows
     into `denyAffordance`'s audit `Summary` but NOT into the wire
     response. Audit consumers see the full attribution chain;
     external clients see only the wire-stable rule_id. Pinned by
     test.

### Critical added in crit round 1

- **DR-C6: `has_role` needs the entity for local (entity-scoped)
  roles.** The draft declared `has_role(user, role_name)` —
  global-only. But the ACL four-layer model
  (`.ignored/acl-design.md`) has *local roles* (`owner`, `editor`,
  `reviewer`) conferred per-entity by a `role_relations` edge
  (`alice --owner-of--> TKT-001`). "Is current_user an owner?" is
  meaningless without "owner of WHAT." A global-only `has_role`
  could never express the most common affordance ("owners may edit
  internal notes"). **Fix applied:** `has_role` is now 3-arg —
  `has_role(current_user, entity, role_name) bool` — and the
  resolver computes the `(principal, entity)` effective role set
  (global ∪ direct local roles conferred on that entity) once per
  call. Local-role grant blocks (`roles.owner.fields.ticket`)
  participate in the same per-role-per-type union (DR-S3),
  naturally entity-scoped. Inherited local roles (via
  `inherit_roles_through`, depth-capped graph walk) are **deferred
  to ACL v1** — v1 of this ticket resolves direct local roles
  only; documented in scope.

- **DR-C7: Global-role checks (e.g. gating a "create" button)
  need an entity-independent form.** Follow-up to DR-C6 (crit
  round 2): once `has_role` is entity-scoped, how does a predicate
  express a purely global-role check where no entity applies?
  **Fix applied:** added a distinct `has_global_role(current_user,
  role_name) bool` rather than overloading `has_role` with a
  nullable entity. Two clearly-typed funcs avoid the nil-entity
  footgun the design review flagged. `has_role` remains the
  superset (global ∪ local); `has_global_role` is the convenience
  form for global-only. **Scope note:** in THIS ticket every
  predicate is per-entity (field/relation affordances attach only
  to per-entity GET; the `create` verb is already gated by the
  phase-1 `_actions` / `acl.AuthorizeWrite` path, not by the field
  resolver). Collection-scope predicates that gate `create` before
  an entity exists are a future ticket — when they land they'll
  declare an env WITHOUT `entity`, so only `has_global_role` is
  available and no nil entity is ever constructed.

### Significant (addressed)

- **DR-S1: Metamodel hot-reload not handled.** `cmd/rela-server`'s
  `app.StartWatching()` may reload the metamodel without rebuilding
  the resolver. **Fix applied:** document "**adding properties
  referenced by predicates requires server restart**" in
  `docs/security.md`. Add a regression test: after metamodel
  reload, the resolver still uses the original RecordType; predicates
  referencing newly-added properties don't auto-compile. If the
  watcher's reload path turns out to rebuild the resolver (read
  `app.StartWatching()` to confirm during implementation), update
  the docs accordingly. v1 explicitly does not support live
  predicate reload.

- **DR-S2: Compile-error collection should be multi-error, not
  first-wins.** **Fix applied:** `affordances.New` uses
  `errors.Join` to collect ALL compile errors and renders each as
  `roles.<role>.<block>.<type>[<i>].when: <err>`. Fail-fast at the
  resolver level; show every failure operators need to fix in one
  pass.

- **DR-S3: Cross-role union under opt-in is non-monotonic.** The
  draft's per-type opt-in meant "more roles can give less access."
  **Fix applied:** redefine as **per-role-per-type opt-in,
  evaluated independently, unioned**:
  - For each (user, type), iterate the user's roles.
  - Each role that declared `fields: {T: [...]}` is closed-world
    *for that role's contribution*.
  - A role without a `fields:` block contributes the empty
    writability set (it doesn't grant per-field writability, but
    it also doesn't shrink other roles' grants).
  - Union the per-role "writable" sets.
  - The role's type-level `write:` still authorizes the verb at
    `acl.AuthorizeWrite` time; the per-field gate fires *additionally*
    via `_fields.writable=false`.

  Result: adding a role monotonically adds access. Added 4 cross-
  role union tests to pin behavior with mixed declared/undeclared
  `fields:` blocks per role.

### Significant (deferred with rationale)

- **DR-S4: Predicates evaluate against full entity, including
  visibility-stripped properties.** Documented as intentional —
  predicates server-side see plaintext; wire response strips. Added
  to security.md.
- **DR-S5: Host-func snapshot scope.** Snapshot once per
  `Resolver.FieldVerdicts` call; pass to host funcs by binding.
  Bound by predicate's step budget (10k ticks; host calls are one
  tick each per `predicate/eval.go:122-157`). Add perf microbench
  in implementation: 50 grants × 5 roles × 3 relation-walks per
  resolver call, assert <10ms.
- **DR-S6: `FieldGrant` and `RelationMetaGrant` are byte-identical.**
  Unified as a single `FieldGrant` type used in both contexts.
- **DR-S7: `*bool` YAML edge cases.** Test fixtures cover explicit
  true/false, absent, null, empty-value, string `"false"`, integer
  `0`. Considered enum alternative (`grant`/`deny`) but `*bool` with
  explicit edge-case tests is simpler and the YAML semantics are
  pinned.
- **DR-S8: Integration tests vs unit tests.** Added
  `cmd/rela-server/main_affordances_test.go`: load a project dir
  with `acl.yaml` carrying affordance blocks, drive HTTP GET +
  PATCH, assert response shape and 403 wiring. This closes the
  end-to-end gap.
- **DR-S9: Missing properties bind as `nil`.** Documented; added
  edge-case test for predicate referencing optional unset property.

### Minor / leverage (acknowledged for implementation)

- **DR-M1: `default` role under per-type opt-in.** Documented:
  adding `fields:` to `default` makes the world closed-world for
  that type. Test pins this surprising-but-correct behavior.
- **DR-M3: `RELA_AFFORDANCE_PROFILE=none` precedence.** Treat
  `none` as a hard override to `NopFieldVerdictResolver`, even
  when policy has affordance blocks. Allows operators to opt out
  explicitly.
- **DR-M4: Asymmetric malformed-YAML (warn) vs malformed-predicate
  (exit).** Documented in `docs/security.md`: predicate compile
  errors are operator typos with deterministic fixes; surface
  loudly.
- **DR-M5: Test fixture location.**
  `internal/acl/affordances/testdata/` for unit-level fixtures,
  `cmd/rela-server/testdata/affordances-project/` for end-to-end.
- **DR-M6: Audit `Summary` format pinning.** Define
  `Summary` format as space-separated `key=value` pairs (e.g.
  `denied: rule_kind=affordance rule_id=... attribution=role=...`)
  and add a regex-based parsing test.
- **DR-L1: Adapter-pattern predicate runtime out of the resolver.**
  Internal `predicateRunner` interface in
  `internal/acl/affordances/` separates resolver from
  predicate-engine specifics. Tests can stub `predicateRunner`
  without standing up the predicate type system.
- **DR-L3: Audit-log invariant integration test.** Walk every
  denial kind, grep audit JSONL, assert one row per denial with
  full attribution.
- **DR-L4: Lint test for `AffordanceDenialError` construction.**
  Add to `internal/dataentry/lint_test.go`: no direct construction
  outside the two owning packages.

Deferred to follow-ups (not addressed in this ticket):

- **DR-L2: Verdict caching.** Documented as future work; resolver
  API shape doesn't preclude adding a `Cache` collaborator.
