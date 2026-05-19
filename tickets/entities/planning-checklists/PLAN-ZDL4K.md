---
id: PLAN-ZDL4K
type: planning-checklist
title: 'Planning: ACL v0: declarative write-side enforcement with delegate-X tamper resistance'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Delivery Plan — three PRs

PR 1 establishes the contract end-to-end with two trivial implementations
(`NopACL` for backwards-compat, `ReadOnlyACL` as a useful real feature). PR 2
lands the policy logic in isolation. PR 3 wires policy loading into the entry
points.

| PR | Title | Scope | Lines (approx) | Risk |
|---|---|---|---|---|
| 1 | ACL interface + NopACL + ReadOnlyACL + Manager wiring + 403 | All scaffolding: interface, `Decision`, `WriteRequest`, `ForbiddenError`. `NopACL` (default, allow-all). `ReadOnlyACL` (deny-all). `--read-only` flag on `rela-server`. `Manager.authorizeAndAudit` helper called from all 7 write paths. `audit.OpDeniedWrite`. Data-entry 403 mapping. Rebaseline existing tests with `NopACL`. | ~400 added, ~50 changed | Low (NopACL default = no behavior change) |
| 2 | Declarative ACL + Policy loading | `internal/acl/declarative.go`, `policy.go`, YAML loader, unit tests. NOT wired anywhere. PR 1's interface already exists. | ~300 added | Very low (no production wiring) |
| 3 | Wire `acl.yaml` into `appbuild`, non-loopback warning, docs | `appbuild` loads `acl.yaml`, falls back to `NopACL` when missing. `rela-server` warns when non-loopback + NopACL. `docs/security.md` ACL section. `docs/audit-log.md` `denied-write` mention. | ~150 added, ~30 changed | Medium (first end-to-end policy evaluation) |

Each PR transitions the ticket through review independently; only PR 3 marks the
ticket `done`. Acceptance criteria below are split per-PR.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**IN scope (v0 ticket, across 3 PRs):**

- Declarative `acl.yaml` schema at project root: `user_entity_type`, `roles`, `assignments`, `role_relations`.
- New `internal/acl` package with three implementations: `Declarative`, `NopACL`, `ReadOnlyACL`.
- Write-side enforcement: every `entitymanager.Manager.{Create,Update,Delete}{Entity,Relation}` and `RenameEntity` consults the ACL before persisting.
- Delegate-X tamper resistance: writing role-conferring relations requires holding the matching `delegate-X` permission.
- Structured HTTP 403 deny error from the data-entry handler: `{error, rule_kind, rule_id, reason}`.
- `--read-only` CLI flag on `rela-server` (and `RELA_READ_ONLY=1` env) → wires `ReadOnlyACL`.
- Audit integration: new `denied-write` op records denied attempts.
- Backwards-compatible: projects without `acl.yaml` get `NopACL` (allow-all); a startup warning fires only when the server binds non-loopback AND `acl.yaml` is absent.
- Wiring: `entitymanager.Deps.ACL` is required (constructor rejects nil).

**OUT of scope (deferred):**

- Read filtering, property redaction, `filtered_by_acl` count (v1).
- Groups and `member-of` transitive resolution (v1).
- MCP transport-layer intersection (v1).
- Containment inheritance via `inherit_roles_through` (v2).
- Per-property `except_properties` (v2).
- Explicit-deny rules block (v2).
- Documented automation-as-escape-hatch patterns (v3).
- Store-neutral `Querier` DSL (v1 — only relevant for read filtering).
- `--read-only` on CLI / scheduler / desktop (v0 keeps it server-only).

**Acceptance Criteria:**

### PR 1 acceptance

1. **AC1.1 — Interface and error types.** `internal/acl` package defines `ACL`, `Decision`, `WriteRequest`, `Op`, `ForbiddenError`. `ForbiddenError` satisfies `errors.Is(err, ErrForbidden)`.
   - **Test:** `internal/acl/acl_test.go::TestForbiddenError_IsErrForbidden`.

2. **AC1.2 — NopACL allows everything.** `NopACL{}.AuthorizeWrite(any)` returns `{Allow: true}`.
   - **Test:** `internal/acl/nop_test.go::TestNopACL_AllowsAllWrites`.

3. **AC1.3 — ReadOnlyACL denies everything.** `ReadOnlyACL{}.AuthorizeWrite(any)` returns `{Allow: false, RuleKind: "read-only", RuleID: "read-only-acl", Reason: "this rela instance is configured read-only"}`.
   - **Test:** `internal/acl/readonly_test.go::TestReadOnlyACL_DeniesAllWrites`.

4. **AC1.4 — `entitymanager.Deps.ACL` is required.** `entitymanager.New` returns an error when `Deps.ACL == nil`. All existing tests pass after threading `acl.NopACL{}` through the fixture.
   - **Test:** `internal/entitymanager/manager_test.go::TestNew_NilACL_Errors`; existing tests in the package green with the rebaseline.

5. **AC1.5 — All 7 write entry points call `authorizeAndAudit` first.** `CreateEntity`, `UpdateEntity`, `DeleteEntity`, `RenameEntity`, `CreateRelation`, `UpdateRelation`, `DeleteRelation` consult the ACL before any store mutation. On deny: returns `*acl.ForbiddenError`, audit log gets a `denied-write` row, no store calls happen.
   - **Test:** `internal/entitymanager/acl_test.go` — table-driven across 7 entry points using `ReadOnlyACL` as the deny fixture and `audit.Memory` to assert on records. Each row: op kind, expected store calls (= 0), expected audit op (= `denied-write`).

6. **AC1.6 — `audit.OpDeniedWrite` constant.** Added as `"denied-write"`.
   - **Test:** referenced by AC1.5; no separate test.

7. **AC1.7 — Data-entry maps `ForbiddenError` to HTTP 403.** Body: `{"error":"forbidden","rule_kind":"...","rule_id":"...","reason":"..."}`. `Content-Type: application/json`.
   - **Test:** `internal/dataentry/acl_test.go::TestHandler_ACLDeny_Returns403Structured` — uses `ReadOnlyACL` against a real `Manager`, hits a write endpoint via `httptest`, asserts on status and body.

8. **AC1.8 — `--read-only` flag wires `ReadOnlyACL`.** `rela-server --read-only` rejects every write attempt across the API. `RELA_READ_ONLY=1` env var works equivalently. Without the flag, behavior is unchanged (NopACL default).
   - **Test:** `cmd/rela-server/main_acl_test.go::TestReadOnlyFlag_WiresReadOnlyACL` — boots the server with the flag, sends a write request, asserts 403 with the read-only reason.

### PR 2 acceptance

9. **AC2.1 — Policy schema & loading.** `acl.yaml` parses into a typed `acl.Policy`. Unknown top-level keys → `slog.Warn`, not error. Missing file → `os.ErrNotExist` from the loader (the caller decides what to do with it).
   - **Test:** `internal/acl/policy_test.go::TestLoadPolicy_*` (Empty, FullExample, UnknownKey_LogsWarning, MissingFile_ReturnsErrNotExist).

10. **AC2.2 — Type-level write grant (allow).** Given role `contributor: {write: [ticket]}` assigned to `alice`, `Declarative.AuthorizeWrite({Op: create, EntityType: ticket})` returns `{Allow: true, RuleKind: role-grant, RuleID: contributor}`.
    - **Test:** `internal/acl/declarative_test.go::TestAuthorizeWrite_RoleGrantsType_Allows`.

11. **AC2.3 — Type-level write deny.** Role with no write on the target type → `{Allow: false, RuleKind: role-grant, RuleID: "-", Reason: "no role grants write on type 'ticket'"}`.
    - **Test:** `TestAuthorizeWrite_NoRoleGrants_Denies`.

12. **AC2.4 — Wildcard write.** `admin: {write: ["*"]}` allows writes to any type.
    - **Test:** `TestAuthorizeWrite_WildcardRole_Allows`.

13. **AC2.5 — Delegate-X tamper resistance.** Given `role_relations.ticket-owner.requires_permission = delegate-contributor`, a user holding only `delegate-reviewer` writing a `ticket-owner` relation gets `{Allow: false, RuleKind: delegate-permission, RuleID: delegate-contributor}`. A user holding `delegate-contributor` is allowed (assuming a role-grant for the entity type would otherwise pass).
    - **Test:** `TestAuthorizeWrite_RoleRelation_DelegatePermission*`.

14. **AC2.6 — `default` role applies.** A principal with no `assignments` entry gets the `default` role's capabilities (if defined).
    - **Test:** `TestAuthorizeWrite_UnknownPrincipal_GetsDefaultRole`.

15. **AC2.7 — Most-permissive union.** A principal with multiple roles gets the union of their writes.
    - **Test:** `TestAuthorizeWrite_MultipleRoles_Unions`.

### PR 3 acceptance

16. **AC3.1 — `appbuild` loads `acl.yaml`.** `appbuild.Discover` and `appbuild.New` load `acl.yaml` from project root and pass the resulting `Declarative` (or `NopACL` on absence) into `entitymanager.Deps`.
    - **Test:** `internal/appbuild/appbuild_acl_test.go::TestDiscover_ACLPresent_LoadsDeclarative`, `TestDiscover_ACLMissing_UsesNop`.

17. **AC3.2 — `--read-only` continues to win over `acl.yaml`.** When both `--read-only` is set AND `acl.yaml` is present, `ReadOnlyACL` wins.
    - **Test:** `TestReadOnlyFlag_OverridesPolicy`.

18. **AC3.3 — Non-loopback warning.** Starting `rela-server --bind 0.0.0.0` without `acl.yaml` (and without `--read-only`) emits one `slog.Warn`. Loopback bind without `acl.yaml` is silent. With `acl.yaml` present, no warning.
    - **Test:** `cmd/rela-server/main_acl_test.go::TestStartup_NonLoopbackWithoutACL_Warns`.

19. **AC3.4 — Snapshot semantics (optional, deferred if not needed).** A principal's effective roles are resolvable once per request. v0 implements it as a no-cache passthrough; the API is in place for v1 to add caching without touching call sites.
    - **Test:** deferred; the v1 group-expansion PR will add the cache and its test.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Cross-system survey** documented in `.ignored/acl-design.md` (Plone, Casbin, OpenFGA/Zanzibar, Cerbos, Oso, Postgres RLS, AWS IAM, Django Guardian, Apache Ranger, Neo4j). Twelve design lessons folded into the design.
- **Casbin** considered and rejected: heavier dependency, perf concerns at scale, glue ≈ size of our own ACL.
- **Patterns in codebase:**
  - `internal/audit/` — same shape we want: small interface, three backends (Nop/Memory/Filesystem), required Deps field, nil-rejecting constructor. We mirror this.
  - `internal/principal/` — already provides `Principal{User, Tool}` and ctx plumbing (TKT-WEBI). ACL reads `principal.From(ctx)` like audit does.
  - `internal/entitymanager/manager.go` — write entry points: `CreateEntity`, `UpdateEntity`, `DeleteEntity`, `RenameEntity`, `CreateRelation`, `UpdateRelation`, `DeleteRelation`. Each gets a single ACL check.
  - `internal/appbuild/appbuild.go` — wiring site for `entitymanager.Deps`. ACL loading lives here in PR 3.

**Python prototype:** `.ignored/acl-prototype/` validated the design end-to-end
including v1's read path. 4 scenarios pass; query-count instrumentation confirms
the cost model.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Package layout (final, after all 3 PRs)

```
internal/acl/
  acl.go              // ACL interface, Decision, WriteRequest, Op, ForbiddenError    (PR 1)
  acl_test.go                                                                          (PR 1)
  nop.go              // NopACL — allow-all                                            (PR 1)
  nop_test.go                                                                          (PR 1)
  readonly.go         // ReadOnlyACL — deny-all                                        (PR 1)
  readonly_test.go                                                                     (PR 1)
  policy.go           // Policy struct + YAML loader                                   (PR 2)
  policy_test.go                                                                       (PR 2)
  declarative.go      // production ACL: composes Policy + reads principal from ctx   (PR 2)
  declarative_test.go                                                                  (PR 2)
```

### Core types — PR 1

```go
// internal/acl/acl.go

package acl

import (
    "context"
    "errors"
    "fmt"
)

type ACL interface {
    AuthorizeWrite(ctx context.Context, req WriteRequest) Decision
}

type WriteRequest struct {
    Op           Op       // OpCreate | OpUpdate | OpDelete | OpRename
    EntityType   string   // type of entity being acted on. For relation writes,
                          // this is the *source* entity's type.
    RelationType string   // populated for relation writes; "" for entity writes.
                          // If RelationType is also a role_relations key, the
                          // delegate-X check fires.
}

type Op string

const (
    OpCreate Op = "create"
    OpUpdate Op = "update"
    OpDelete Op = "delete"
    OpRename Op = "rename"
)

type Decision struct {
    Allow    bool
    RuleKind string  // "role-grant" | "delegate-permission" | "read-only"
    RuleID   string  // role name, permission name, or "read-only-acl"; "-" when no rule applied
    Reason   string  // human-readable, surfaces in 403 body and audit log
}

var ErrForbidden = errors.New("forbidden")

type ForbiddenError struct {
    Decision Decision
}

func (e *ForbiddenError) Error() string {
    return fmt.Sprintf("forbidden: %s (rule_kind=%s rule_id=%s)",
        e.Decision.Reason, e.Decision.RuleKind, e.Decision.RuleID)
}

func (e *ForbiddenError) Is(target error) bool { return target == ErrForbidden }
```

### NopACL — PR 1

```go
// internal/acl/nop.go

package acl

import "context"

// NopACL is the explicit opt-out: allows every write. Wired by appbuild when
// no acl.yaml is present so projects that don't care about access control
// run unchanged.
type NopACL struct{}

func (NopACL) AuthorizeWrite(_ context.Context, _ WriteRequest) Decision {
    return Decision{Allow: true}
}
```

### ReadOnlyACL — PR 1

```go
// internal/acl/readonly.go

package acl

import "context"

// ReadOnlyACL denies every write with a single fixed Decision. Useful for:
//   - Operating rela-server in observe-only mode for demos or maintenance.
//   - Exercising the full deny path (HTTP 403, audit denied-write) without
//     a policy file.
//   - A fail-safe an operator can wire when they want absolute confidence
//     no writes happen.
//
// Wired via `rela-server --read-only` or RELA_READ_ONLY=1.
type ReadOnlyACL struct{}

func (ReadOnlyACL) AuthorizeWrite(_ context.Context, _ WriteRequest) Decision {
    return Decision{
        Allow:    false,
        RuleKind: "read-only",
        RuleID:   "read-only-acl",
        Reason:   "this rela instance is configured read-only",
    }
}
```

### `authorizeAndAudit` helper — PR 1

Small refactor to keep the 7 write entry points consistent:

```go
// internal/entitymanager/manager.go (new helper)

// authorizeAndAudit consults the ACL and, on deny, records a denied-write
// audit row and returns *acl.ForbiddenError. On allow, returns nil and the
// caller proceeds. Called as the first non-validation step in every write
// entry point.
func (m *Manager) authorizeAndAudit(ctx context.Context, req acl.WriteRequest) error {
    decision := m.deps.ACL.AuthorizeWrite(ctx, req)
    if decision.Allow {
        return nil
    }
    m.recordDeniedWrite(ctx, decision, req)
    return &acl.ForbiddenError{Decision: decision}
}

func (m *Manager) recordDeniedWrite(ctx context.Context, d acl.Decision, req acl.WriteRequest) {
    var subject *audit.Subject
    if req.RelationType != "" {
        subject = &audit.Subject{Kind: "relation", RelationType: req.RelationType}
    } else {
        subject = &audit.Subject{Kind: "entity", Type: req.EntityType}
    }
    m.deps.Audit.Record(audit.Record{
        Time:        time.Now().UTC(),
        Op:          audit.OpDeniedWrite,
        Subject:     subject,
        Principal:   principal.From(ctx),
        TriggeredBy: audit.TriggeredByFrom(ctx),
        Summary: fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s) attempted op=%s",
            d.Reason, d.RuleKind, d.RuleID, req.Op),
    })
}
```

Each write entry point gains one line near the top:

```go
func (m *Manager) CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error) {
    if e == nil { return nil, errors.New("...") }
    if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
        Op: acl.OpCreate, EntityType: e.Type,
    }); err != nil {
        return nil, err
    }
    // ... existing logic
}
```

For relations, the request carries source-entity type + relation type:

```go
fromEntity, err := m.deps.Store.GetEntity(ctx, from)
if err != nil { return nil, fmt.Errorf("source %w: %s", ErrEntityNotFound, from) }
toEntity, err := m.deps.Store.GetEntity(ctx, to)
if err != nil { return nil, fmt.Errorf("target %w: %s", ErrEntityNotFound, to) }
if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
    Op: acl.OpCreate, EntityType: fromEntity.Type, RelationType: relType,
}); err != nil {
    return nil, err
}
```

Note for relations: source/target lookup happens *before* the ACL check so the
ACL has a real entity type. Trade-off: a deny costs 2 store gets we'd skip if we
denied first. Accepted because the alternative (denying writes to non-existent
entities without a clear reason) is worse UX.

### Data-entry 403 mapping — PR 1

Need to find where existing handler errors get mapped. Check
`internal/dataentry/router.go` and surrounding files during PR 1 implementation.
The mapping:

```go
var forbidden *acl.ForbiddenError
if errors.As(err, &forbidden) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusForbidden)
    json.NewEncoder(w).Encode(map[string]string{
        "error":     "forbidden",
        "rule_kind": forbidden.Decision.RuleKind,
        "rule_id":   forbidden.Decision.RuleID,
        "reason":    forbidden.Decision.Reason,
    })
    return
}
```

### `--read-only` flag — PR 1

```go
// cmd/rela-server/main.go
readOnly := flag.Bool("read-only", false, "Refuse all writes. Useful for demos, maintenance, observe-only mode.")
// ... after flag.Parse() ...
if os.Getenv("RELA_READ_ONLY") == "1" {
    *readOnly = true
}
// ... when constructing Services ...
var aclImpl acl.ACL = acl.NopACL{}  // default; PR 3 changes this to load from acl.yaml
if *readOnly {
    aclImpl = acl.ReadOnlyACL{}
}
```

PR 3 changes the default branch to load `acl.yaml`; the `--read-only` override
stays.

### PR 2 — Declarative + Policy (sketch)

```go
// internal/acl/policy.go

type Policy struct {
    UserEntityType string                     `yaml:"user_entity_type"`
    Roles          map[string]RoleDef         `yaml:"roles"`
    Assignments    map[string]string          `yaml:"assignments"`
    RoleRelations  map[string]RoleRelationDef `yaml:"role_relations"`
}

type RoleDef struct {
    Write       []string `yaml:"write"`
    Read        []string `yaml:"read"`        // parsed but unused in v0
    Permissions []string `yaml:"permissions"`
}

type RoleRelationDef struct {
    Confers            string `yaml:"confers"`
    RequiresPermission string `yaml:"requires_permission"`
}

func LoadPolicy(path string) (*Policy, error)
```

```go
// internal/acl/declarative.go

type Declarative struct {
    policy *Policy
}

func NewDeclarative(p *Policy) *Declarative { return &Declarative{policy: p} }

func (d *Declarative) AuthorizeWrite(ctx context.Context, req WriteRequest) Decision {
    // 1. Delegate-X check for role-relation writes.
    // 2. Type-level write grant from effective roles.
    // 3. Deny.
}
```

### PR 3 — Wiring

```go
// internal/appbuild/appbuild.go

func loadACL(paths *project.Context, readOnly bool) (acl.ACL, error) {
    if readOnly {
        return acl.ReadOnlyACL{}, nil
    }
    path := filepath.Join(paths.Root, "acl.yaml")
    policy, err := acl.LoadPolicy(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return acl.NopACL{}, nil
        }
        return nil, err
    }
    return acl.NewDeclarative(policy), nil
}
```

`rela-server --bind <non-loopback>` + NopACL → `slog.Warn`.

### Files modified per PR

**PR 1:**

| File | Change |
|---|---|
| `internal/acl/acl.go` | NEW |
| `internal/acl/acl_test.go` | NEW |
| `internal/acl/nop.go` | NEW |
| `internal/acl/nop_test.go` | NEW |
| `internal/acl/readonly.go` | NEW |
| `internal/acl/readonly_test.go` | NEW |
| `internal/audit/audit.go` | Add `OpDeniedWrite` constant |
| `internal/entitymanager/entitymanager.go` | Add `ACL` to `Deps`; nil-reject in `New` |
| `internal/entitymanager/manager.go` | Add `authorizeAndAudit` + `recordDeniedWrite`; call from 7 write paths |
| `internal/entitymanager/acl_test.go` | NEW — table-driven across 7 entry points |
| `internal/entitymanager/manager_test.go` and other tests in this package | Thread `acl.NopACL{}` through |
| `internal/entitymanager/entitymanagertest/` | Add `NopACL` to fixture |
| `internal/dataentry/router.go` (or new errors.go) | Map `acl.ForbiddenError` → 403 structured body |
| `internal/dataentry/acl_test.go` | NEW |
| `cmd/rela-server/main.go` | `--read-only` flag + env, wire `NopACL` (default) or `ReadOnlyACL` |
| `cmd/rela-server/main_acl_test.go` | NEW |
| `.go-arch-lint.yml` | Declare `acl` component, allowed imports |
| `internal/appbuild/appbuild.go` | Accept `ACL` for `entitymanager.Deps` (just pass through; default `NopACL`) |

**PR 2:**

| File | Change |
|---|---|
| `internal/acl/policy.go` | NEW |
| `internal/acl/policy_test.go` | NEW |
| `internal/acl/declarative.go` | NEW |
| `internal/acl/declarative_test.go` | NEW |

**PR 3:**

| File | Change |
|---|---|
| `internal/appbuild/appbuild.go` | Call `loadACL`, pick `Declarative` when `acl.yaml` present |
| `internal/appbuild/appbuild_acl_test.go` | NEW |
| `cmd/rela-server/main.go` | Non-loopback + NopACL → `slog.Warn` |
| `cmd/rela-server/main_acl_test.go` | Extend with warning test |
| `docs/security.md` | ACL section: schema, delegate-X, trust model, `--read-only` |
| `docs/audit-log.md` | Document `denied-write` op |
| `CLAUDE.md` | Brief note about the ACL package and "Lua never on read path" |

**Alternatives considered:**

| Alternative | Rejected because |
|---|---|
| Single big PR | Higher review risk; harder to roll back if integration goes wrong |
| Two PRs (skeleton + everything else) | "Everything else" still too large; loses the policy-isolation-from-wiring split |
| Skip `ReadOnlyACL` | Loses the most valuable PR 1 outcome — demonstrable feature without policy logic |
| Use Casbin | Heavier dep; perf concerns; glue ≈ size of our own ACL |
| Embed ACL in entitymanager | Violates CLAUDE.md "consumer-side interfaces" |
| Lua-based policy from day one | Rejected per design doc Finding 5 |

**Dependencies:**

- `gopkg.in/yaml.v3` (already in go.mod)
- `internal/principal`, `internal/audit`, `internal/entitymanager`, `internal/appbuild` (all existing)
- No new third-party dependencies.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `acl.yaml` content | Project filesystem (PR-reviewed) | YAML parse; allowlist of known keys; warn-not-fail on unknown keys | YAML parse error → startup fails loudly |
| `principal.User` | HTTP header via `PrincipalResolver` (TKT-WEBI sanitizes already) | Already trimmed, length-capped at 256, control-chars stripped | "unknown" by fall-through |
| `WriteRequest.EntityType` / `RelationType` | Caller-supplied from entitymanager | Trusted (from validated entity/relation creation paths) | n/a |
| `--read-only` flag | CLI / env | Boolean, no parsing concern | n/a |
| Role names in `assignments` | YAML | Implicit allowlist: only roles in `roles:` produce grants; unknown role names log warning and are ignored | Warning at load |
| Permission names in `requires_permission` | YAML | Cross-checked against `roles[*].permissions` at load; warn if no role grants it | Warning at load |

**Security-Sensitive Operations:**

| Operation | Protection |
|---|---|
| Loading `acl.yaml` | File-system trust (same as `metamodel.yaml`); not user-editable at runtime |
| Granting role-conferring relations | `delegate-X` permission check (Plone pattern) |
| Audit recording of denied writes | Uses existing audit infrastructure; never blocks the deny response |
| 403 error body | Names rule_kind, rule_id, reason — but not full policy contents |
| `--read-only` override | Defense-in-depth; cannot be disabled at runtime once set |

**Error handling — no info leaks:**

- Deny reasons are constructed messages, not raw policy paths or assignment maps.
- The 403 body includes only `rule_kind`, `rule_id`, `reason` — never the full effective-role set.
- `slog.Warn` for non-loopback + missing-ACL stays operator-facing.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Mapped per-PR above. Summary:

- **PR 1**: `internal/acl` unit tests (NopACL allow-all, ReadOnlyACL deny-all, ForbiddenError errors.Is); `internal/entitymanager/acl_test.go` integration tests across 7 entry points using ReadOnlyACL fixture; `internal/dataentry/acl_test.go` end-to-end 403 test; `cmd/rela-server` startup test for `--read-only`.
- **PR 2**: `internal/acl/policy_test.go` (load paths) and `declarative_test.go` (table-driven authorize cases).
- **PR 3**: `internal/appbuild/appbuild_acl_test.go` (wiring) and `cmd/rela-server` warning test.

**Edge Cases:**

- Empty `acl.yaml` (`---` only) → zero `Policy`; matches no-`acl.yaml` behavior (PR 2 test).
- `acl.yaml` with `roles:` but no `assignments:` → everyone gets `default` only (PR 2 test).
- `assignments` references an undefined role → warn at load, drop entry, principal falls through to `default` (PR 2 test).
- `roles.X.write` contains `"*"` → wildcard (PR 2 test).
- Principal user is empty string → treated as "unknown" (already sanitized upstream; no new test needed).
- `--read-only` + `acl.yaml` both set → `ReadOnlyACL` wins (PR 3 test AC3.2).
- `RELA_READ_ONLY=1` + `--read-only=false` (env contradicts flag) → env wins (env-or-flag, not strict precedence). Document in flag help.
- Concurrent writes by same principal → no shared state, safe.
- Audit fails during denied-write recording → deny still returns 403; audit backend logs via slog.

**Negative Tests:**

- Policy YAML with bad indentation → parse fails, error names line/col (PR 2).
- `WriteRequest{EntityType: "", RelationType: ""}` → declarative returns deny with "no role grants write on type ''" (PR 2).
- `entitymanager.New` with `Deps.ACL == nil` → returns error (PR 1, AC1.4).
- Multiple roles per user via list (`assignments: {alice: [a, b]}`) → YAML parse rejects (schema is `map[string]string`) (PR 2).

**Integration test approach:**

- PR 1's `entitymanager/acl_test.go` uses real `ReadOnlyACL` + `audit.Memory` + the existing `entitymanagertest` fixture so denied writes are observable in both error and audit records.
- PR 1's `dataentry/acl_test.go` uses `httptest.NewRecorder` + a real `Manager` with `ReadOnlyACL`, asserting on response status code AND JSON body shape.
- PR 3's `appbuild_acl_test.go` writes a temp `acl.yaml`, calls `appbuild.Discover`, asserts the returned `Services` has a `Declarative` ACL.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort:** `l` (large) — confirmed. Split across 3 PRs: ~3 + ~2 + ~2 working
days.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| Manager integration ripples through many tests | High | Threaded through `entitymanagertest` fixture in PR 1; existing tests get `NopACL` for free. The 631-line `audit_test.go` likely needs one constructor-args change. |
| Non-trivial config surface — `acl.yaml` schema | High | Schema kept tiny in v0 (4 top-level keys); load-time warnings catch typos; PR 2 owns schema in isolation. |
| `acl.yaml` location ambiguity | Medium | Decision: project root, alongside `metamodel.yaml`. Documented in PR 3's `docs/security.md` update. |
| Cache-once semantics tricky | Medium | v0 ships without cache; resolution is cheap and per-request. Defer until profiler shows it matters. |
| `audit.OpDeniedWrite` breaks downstream JSONL readers | Low | Audit JSONL is forensic, not authoritative; adding new ops is non-breaking. Documented in PR 3's `docs/audit-log.md` update. |
| `appbuild` change pulls ACL loading into every entry point | Medium | Each entry point's ACL is the same `Declarative`; CLI/MCP/scheduler get same behavior as data-entry. `--read-only` is server-only. |
| `--read-only` flag accidentally becomes a footgun (operator sets it without intending) | Low | Startup log line confirms read-only mode; `--help` flag text says "refuses all writes". |
| Test coverage thresholds for new packages | Low | Standard: add `internal/acl` to coverage floor. Coverage will exceed 90% given the test plan. |
| Inter-PR merge conflicts (PR 2 lands on top of PR 1) | Low | PR 2 touches only `internal/acl/policy.go` and `declarative.go` — files PR 1 doesn't touch. PR 3 touches different files again. Conflict-free if landed in order. |

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — **YES** (PR 3): `docs/security.md` ACL section: schema, delegate-X, trust boundary, `--read-only`, no-ACL default.
- [x] CLI help text — **YES** (PR 1): `rela-server --read-only` flag help.
- [x] CLAUDE.md — **YES** (PR 3): brief note about ACL package, consumer-side interface pattern applied, "Lua never on read path" discipline.
- [x] ~~README.md~~ (N/A: server-level feature, not project-level)
- [x] ~~API docs~~ (N/A: covered by Go doc comments per package)
- [x] `docs/audit-log.md` — **YES** (PR 3): document `denied-write` op.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: planning already incorporated the cross-system research findings in `.ignored/acl-design.md`, the prototype validation, and the ReadOnlyACL refinement; no separate design-review round was warranted)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design-review round produced findings; the equivalent rigor came from the research sweep folded into `.ignored/acl-design.md`)

**Design Review Findings:** None — see above.
