---
id: TKT-K0C83
type: ticket
title: 'ACL v0 PR 3: Wire acl.yaml into appbuild + non-loopback warning + docs'
kind: enhancement
priority: medium
effort: s
status: done
---

## Scope

PR 3 of the staged ACL v0 delivery. Wires PR 2's `Declarative` into production
via `appbuild.loadACL`, adds the non-loopback warning, and lands the user-facing
docs.

## Acceptance criteria

Cherry-picked from PLAN-ZDL4K (PR 3 ACs):

1. **AC3.1 — `appbuild` loads `acl.yaml`.** `appbuild.Discover` loads `acl.yaml` from project root and passes the resulting `Declarative` (or `NopACL` on absence) into `entitymanager.Deps`.
2. **AC3.2 — `--read-only` continues to win over `acl.yaml`.** Already true via PR 1's `WithACL` option; PR 3 just keeps the precedence intact.
3. **AC3.3 — Non-loopback warning.** `rela-server --bind 0.0.0.0` without `acl.yaml` (and without `--read-only`) emits one `slog.Warn`. Loopback bind is silent. `acl.yaml` present → no warning.

## Files

- `internal/appbuild/appbuild.go` — call `acl.LoadPolicy`, fall back to `NopACL` on `os.ErrNotExist`
- `internal/appbuild/appbuild_acl_test.go` — wiring tests
- `cmd/rela-server/main.go` — non-loopback + NopACL warning
- `docs/security.md` — ACL section: schema, delegate-X, trust model, `--read-only`
- `docs/audit-log.md` — document `denied-write` op
- `CLAUDE.md` — brief note about ACL package + "Lua never on read path" discipline

## Out of scope

- Anything not on PLAN-ZDL4K's PR 3 acceptance list.

## References

- Parent feature: FEAT-AESD4
- Decision: DEC-RG878
- Design: `.ignored/acl-design.md`
- Depends on: PR 2 ticket
