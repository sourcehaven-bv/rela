---
id: PLAN-GYPT
type: planning-checklist
title: 'Planning: ACL v0 PR 2: Declarative ACL + Policy loading (acl.yaml)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Inherits from PLAN-ZDL4K (parent v0 plan). PR 2 specifically:

- `internal/acl/policy.go` — `Policy`, `RoleDef`, `RoleRelationDef`, `LoadPolicy(path)`
- `internal/acl/policy_test.go`
- `internal/acl/declarative.go` — production ACL composing a Policy + reading principal from ctx
- `internal/acl/declarative_test.go`

**Out**: wiring into `appbuild` (PR 3), groups, read filtering, MCP
intersection.

**Acceptance Criteria:** PR 2 ACs are AC2.1–AC2.7 in PLAN-ZDL4K. Repeated here
verbatim for self-containment:

1. **AC2.1** — `acl.yaml` parses to typed Policy; unknown keys → slog.Warn; missing file → os.ErrNotExist.
2. **AC2.2** — Role grants type → Allow with `RuleKind=role-grant`, `RuleID=<role>`.
3. **AC2.3** — No role grants type → Deny with `Reason="no role grants write on type 'X'"`.
4. **AC2.4** — Wildcard `["*"]` grants any type.
5. **AC2.5** — Delegate-X: role-relation write requires the named permission.
6. **AC2.6** — Principal without an assignment gets the `default` role's capabilities.
7. **AC2.7** — Multi-role principal gets the union of writes.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

Inherits from PLAN-ZDL4K research. Python prototype in
`.ignored/acl-prototype/acl.py` already implements the exact AuthorizeWrite
logic this PR ports to Go. The pattern is also exercised end-to-end in the
prototype's `scenarios.py` (tickets + ISMS + PM tool).

Library choice: `gopkg.in/yaml.v3` (already in go.mod, used by metamodel
loader).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### `policy.go`

```go
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

Loader uses `yaml.NewDecoder(f).KnownFields(false)` to tolerate unknown keys (we
warn rather than error). Manual second pass scans the YAML top level for keys
not in the known set and emits one `slog.Warn` per unknown key.

### `declarative.go`

```go
type Declarative struct { policy *Policy }
func NewDeclarative(p *Policy) *Declarative

func (d *Declarative) AuthorizeWrite(ctx context.Context, req WriteRequest) Decision {
    p := principal.From(ctx)
    roles := d.effectiveRoles(p)            // global only — no groups in v0
    perms := d.effectivePermissions(roles)  // union of role.Permissions

    // 1. Delegate-X check (only for relation writes where RelationType matches role_relations)
    if req.RelationType != "" {
        if rr, ok := d.policy.RoleRelations[req.RelationType]; ok {
            if rp := rr.RequiresPermission; rp != "" && !perms[rp] {
                return Decision{Allow: false, RuleKind: "delegate-permission", RuleID: rp, Reason: ...}
            }
        }
    }

    // 2. Type-level write grant (union across roles, first hit wins for RuleID)
    target := req.EntityType
    if target == "" { target = "relation" }
    for _, name := range roles {
        role := d.policy.Roles[name]
        if hasWrite(role, target) {
            return Decision{Allow: true, RuleKind: "role-grant", RuleID: name}
        }
    }
    return Decision{Allow: false, RuleKind: "role-grant", RuleID: "-", Reason: ...}
}

func (d *Declarative) effectiveRoles(p principal.Principal) []string {
    var out []string
    if r, ok := d.policy.Assignments[p.User]; ok { out = append(out, r) }
    if _, ok := d.policy.Roles["default"]; ok { out = append(out, "default") }
    return out
}
```

### Defer to PR 3

- `appbuild.loadACL` (PR 3 wires it).
- Non-loopback warning (PR 3).
- Per-request cache of effective roles (deferred to v1 / when group expansion exists; v0 resolution is two map lookups).

**Files to modify:** only the four new files listed above.

**Alternatives considered:**

| Alternative | Rejected because |
|---|---|
| Eager Policy validation at load (reject undefined role refs) | Defer to `analyze_*` style warnings — same convention as metamodel which tolerates partial state. |
| Cache effective roles on ctx | Premature in v0 (two map lookups, single call per write). v1 group resolution will introduce the cache. |
| Strict YAML decoder (unknown=error) | Operators iterate fast on `acl.yaml`; warn-not-fail matches the metamodel loader's tolerance. |

**Dependencies:** `gopkg.in/yaml.v3` (existing), `internal/principal`
(existing), nothing new.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `acl.yaml` | Project filesystem (PR-reviewed) | YAML parse + known-key allowlist | parse error → return err; unknown key → warn |
| `principal.User` | ctx (TKT-WEBI already sanitized) | n/a | already trimmed/control-stripped upstream |
| `WriteRequest.*` | caller (entitymanager) | trusted internal API | n/a |

**No new sensitive operations.** Reasons constructed in Go, not interpolated
from policy strings.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

| AC | Test name |
|---|---|
| AC2.1 | `TestLoadPolicy_Empty`, `TestLoadPolicy_FullExample`, `TestLoadPolicy_UnknownKey_LogsWarning`, `TestLoadPolicy_MissingFile_ReturnsErrNotExist` |
| AC2.2 | `TestAuthorizeWrite_RoleGrantsType_Allows` |
| AC2.3 | `TestAuthorizeWrite_NoRoleGrants_Denies` |
| AC2.4 | `TestAuthorizeWrite_WildcardRole_Allows` |
| AC2.5 | `TestAuthorizeWrite_RoleRelation_DelegatePermissionMissing_Denies`, `TestAuthorizeWrite_RoleRelation_DelegatePermissionHeld_Allows` |
| AC2.6 | `TestAuthorizeWrite_UnknownPrincipal_GetsDefaultRole` |
| AC2.7 | `TestAuthorizeWrite_MultipleRoles_Unions` |

**Edge cases:** empty Policy (zero value), `Roles` nil, principal=empty string,
wildcard + explicit type both present, requires_permission referencing undefined
permission (warn at load — covered by AC2.1).

**Negative tests:** malformed YAML; assignments referencing undefined role (warn
+ drop); `WriteRequest{}` (empty everything) → "no role grants write on type
''".

**Integration:** PR 2 introduces no wiring — declarative is exercised purely
through unit tests. Integration with Manager comes in PR 3.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort:** `m` (≈2 days).

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| YAML decoder behaviour on unknown keys (strict vs lax) varies by library version | Low | Use known-good `yaml.v3` API; verify with a `TestLoadPolicy_UnknownKey_LogsWarning` test. |
| Surprise interaction with PR 1's `ForbiddenError` shape | Low | Declarative returns the same `Decision` struct PR 1 defined; pure composition. |
| `default` role precedence with explicit assignments | Low | Always appended; union semantics make order irrelevant. Test pins both orders. |

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

PR 2 ships no user-facing docs — operators see nothing until PR 3 wires loading.
Docs for the full schema land in PR 3 (`docs/security.md` ACL section).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design fully covered by PLAN-ZDL4K + Python prototype + crit-approved PR 1)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: none)

**Design Review Findings:** None.
