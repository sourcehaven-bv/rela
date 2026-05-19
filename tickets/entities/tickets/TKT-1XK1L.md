---
id: TKT-1XK1L
type: ticket
title: 'ACL v0 PR 2: Declarative ACL + Policy loading (acl.yaml)'
kind: enhancement
priority: medium
effort: m
status: done
---

## Scope

PR 2 of the staged ACL v0 delivery (parent feature FEAT-AESD4; sibling PR 1 =
TKT-GN5LN).

Adds the policy-driven `Declarative` ACL alongside the `NopACL` / `ReadOnlyACL`
shipped in PR 1. **Not wired into production yet** — PR 3 will load `acl.yaml`
via `appbuild`. This PR is pure policy logic in isolation.

## Acceptance criteria

Cherry-picked from PLAN-ZDL4K (PR 2 ACs):

1. **AC2.1 — Policy schema & loading.** `acl.yaml` parses into a typed `acl.Policy`. Unknown top-level keys → `slog.Warn`. Missing file → `os.ErrNotExist`.
2. **AC2.2 — Type-level write grant (allow).**
3. **AC2.3 — Type-level write deny.**
4. **AC2.4 — Wildcard write.**
5. **AC2.5 — Delegate-X tamper resistance.**
6. **AC2.6 — `default` role applies.**
7. **AC2.7 — Most-permissive union.**

See PLAN-ZDL4K for full text per AC including test names.

## Files (new only — PR 2 touches nothing PR 1 created)

- `internal/acl/policy.go` — `Policy`, `RoleDef`, `RoleRelationDef`, `LoadPolicy(path)`
- `internal/acl/policy_test.go`
- `internal/acl/declarative.go` — `Declarative` ACL implementation
- `internal/acl/declarative_test.go`

## Out of scope

- Wiring into `appbuild` (PR 3 / TKT-???).
- Read filtering (v1).
- Groups + transitive resolution (v1).

## References

- Parent feature: FEAT-AESD4
- Decision: DEC-RG878
- Design: `.ignored/acl-design.md`
- PR 1: TKT-GN5LN
