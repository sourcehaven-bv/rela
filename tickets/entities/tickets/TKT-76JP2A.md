---
id: TKT-76JP2A
type: ticket
title: 'acl: migrate a scheduler read grant into acl.yaml (new FileTypeACL + runner create-path + operator notice)'
kind: enhancement
priority: high
effort: m
status: backlog
---

## Summary

Split out of TKT-ZF2DTV (DEC-O59WM4). That ticket made Lua reads resolve against
the acting identity; scheduled tasks resolve against `system:scheduler` (or
their `run_as`). What it deliberately did NOT do is make reads **fail closed**,
because doing so safely requires writing an explicit grant into existing
deployments first.

## The problem this closes

Today a deployment WITH an `acl.yaml` has no assignment for `system:scheduler`.
Scheduled scripts still read everything only because the read path is permissive
where no grant exists. That is fail-open, and it means the ACL-bound-LLM-job
story ("a job role bounds what enters a prompt") isn't actually enforced yet —
it's honoured by convention.

Writing the grant explicitly lets the runtime flip to **fail closed**: an
identity with no grants reads nothing, and the scheduler's reach becomes visible
in the one place privileges live (inspectable via `rela acl who-can` and the
effective-access map).

## Scope

- New `migration.FileTypeACL` alongside `FileTypeMetamodel` / `FileTypeDataEntry`.
- A migration writing the scheduler grant:
  ```yaml
  roles:
    scheduler-system:
      read: ["*"]
  assignments:
    system:scheduler: scheduler-system
  ```
- **Runner create-path.** `migration.Apply`/`Detect` currently read-then-mutate an existing file (`runner.go:98`, `:161`) and error on a missing one — they have no way to CREATE. Extend them, or add a sibling entry point, so a project without `acl.yaml` gets one.
- **Operator notice (load-bearing).** Creating `acl.yaml` flips a project from `NopACL` (everything permitted for everyone) to declarative (deny-by-default for EVERY principal) — consequential far beyond the scheduler. This must be surfaced loudly, never silent. Consider a dry-run listing before writing.
- **Then** flip script reads to fail closed and drop the permissive fallbacks in `appbuild.scriptEntityReader` / `dataentry.App.scriptReader` (they currently degrade to the raw store with a warning on construction failure — revisit whether that stays once grants are explicit).
- Tests: migration on a project WITH an existing acl.yaml (grant added, file otherwise preserved — comments/formatting intact per the yaml.Node contract); WITHOUT one (created + notice emitted); malformed existing acl.yaml (fail, don't clobber); a scheduled task reading nothing when its identity has no grant.

## Non-goals

Per-job role authoring UX. Changing what `run_as` means (it selects an identity;
privileges stay in acl.yaml).

## References

- `internal/migration/{migration.go:40 (FileType), runner.go:98,161}`
- `internal/appbuild/appbuild.go` (scriptEntityReader), `internal/dataentry/app.go` (scriptReader)
- DEC-O59WM4, TKT-ZF2DTV, PLAN-C3G1VO AC10
