---
id: TKT-76JP2A
type: ticket
title: 'acl: fixed scheduler identity + migrate a read grant into an existing acl.yaml'
kind: enhancement
priority: high
effort: m
status: done
---

## Summary

Split out of TKT-ZF2DTV (DEC-O59WM4). That ticket made Lua reads resolve against
the acting identity; scheduled tasks resolve against the scheduler's identity
(or their `run_as`).

Two defects were found while planning this, both verified against the merged
code rather than assumed. They changed the shape of the work substantially.

## Correction 1: reads already fail closed

The ticket was written assuming reads were fail-open where no grant existed,
and that this ticket would flip them closed. **That was wrong.**

- `acl.readQuery` returns `DenyAll` when no role confers read
  (`internal/acl/readquery.go:49-50`).
- Probe: an unassigned scheduler principal gets `store.ErrNotFound` for an
  entity that exists.

So this is **breaking-change repair, not hardening**. Any deployment with an
`acl.yaml` that runs scheduled tasks has had those tasks silently reading
nothing since TKT-ZF2DTV merged — silently, because a gated read is
indistinguishable from missing data.

## Correction 2: there was no fixed identity to grant to (RR-1USMEZ)

The original plan was to write `assignments: {system:scheduler: ...}`. That
principal **did not exist**. `stampTaskAuditContext` defaulted to
`principal.SystemUser()` = `$USER`:

```text
run_as EMPTY  -> Principal.User = "deploy-bot"   ($USER)
run_as SET    -> Principal.User = "system:digest"
```

The read identity was therefore either operator-chosen (`run_as`) or the OS
account running `rela scheduler` — both unknowable to a static migration. Worse,
a systemd unit typically has no `$USER`, so `SystemUser()` returned the literal
`"unknown"`, which `acl.Declarative.ForPrincipal` **rejects** as an unstamped
principal: those tasks failed outright rather than being scoped.

A migration writing `system:scheduler` would have been a silent no-op.

## What shipped

1. **`principal.UserScheduler = "system:scheduler"`** — a fixed default identity
   for tasks with no `run_as`, replacing `SystemUser()`. Makes the scheduler's
   identity a documented, grantable constant instead of a per-host accident.
2. **`migration.FileTypeACL` + `acl-scheduler-grant`** — adds the role and
   assignment to an **existing** `acl.yaml`, preserving comments and formatting.
3. **Docs** — the `run_as` section now states the default identity, tells
   operators of policy-configured projects to grant it, and notes the audit
   attribution change (scheduled writes now record `system:scheduler` rather
   than the OS account).

## Deliberately NOT done: creating `acl.yaml` (RR-SVQ5HE)

The original instruction was to create the file when absent. Dropped after
verifying it would be net-harmful:

- A project with no `acl.yaml` **has no regression**. `scriptEntityReader`
  returns the raw store when no policy exists (`appbuild.go:254`), confirmed by
  probe — scheduled tasks there read everything and always have.
- Creating a policy flips the project to deny-by-default **for every
  principal**, since `readQuery` keys on the role set, not the identity. The
  generated file grants only the scheduler, so every human, CLI and MCP caller
  loses all reads at once.

The migrate runner's skip-on-missing behavior enforces this, pinned by
`TestMigrate_LeavesPolicylessProjectAlone`.

## Review findings addressed

- **RR-KG2FCX** (critical) — `roles:`/`assignments:` with nothing under them
  parse as a null scalar, and `InsertMapKeyAfter` no-ops on an existing key, so
  the migration reported success while granting nothing. Fixed with a shared
  `EnsureMapping` helper that repairs a null value in place.
- **RR-2ZIGX3** (significant) — `Detect` checked assignment-key existence rather
  than whether the scheduler could actually read, going quiet on dangling and
  read-less roles, and never consulting `asserted_role_assignments`. `Apply` now
  verifies its own postcondition and errors instead of writing a file that
  grants nothing.

## Non-goals

Per-job role authoring UX. Changing what `run_as` means (it selects an identity;
privileges stay in `acl.yaml`). The data-entry fail-open fallback (rela#1198).

## References

- `internal/principal/principal.go` (`UserScheduler`)
- `internal/migration/{acl_scheduler_grant.go, yaml_util.go}`
- `internal/acl/readquery.go:49-50`
- DEC-O59WM4, TKT-ZF2DTV
