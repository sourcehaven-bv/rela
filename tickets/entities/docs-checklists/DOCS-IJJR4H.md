---
id: DOCS-IJJR4H
type: docs-checklist
title: 'Documentation: Per-command ACL guard'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on exported/new identifiers
- [x] Non-obvious decisions explained in comments
- [x] ~~README in new package~~ (N/A: no new package)

`CommandConfig.Permission` (`internal/dataentryconfig/config.go`) carries godoc
covering the bimodal policy, the acl.yaml grant mechanism, and the view
carve-out.

`authorizeCommand` (`internal/dataentry/commands.go`) documents *why* the
read-only arm is checked before the read gate — `nopReadGate.HoldsPermission`
returns true under both `NopACL` and `ReadOnlyACL`, so a guard built on the gate
alone fails open. That is the non-obvious fact the whole design turns on, and it
names its canary test.

The switch's closed-by-construction property is stated as an invariant, not a
request: "Adding a new acl.ACL implementation? It denies commands until you add
an arm."

`runningCommand.owner` explains the cross-principal-kill reasoning rather than
just declaring the field.

## Project Documentation

- [x] `docs/data-entry.md` updated
- [x] `docs/acl-security.md` updated
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~CLAUDE.md~~ (N/A: no new convention — follows the existing named-permission pattern)
- [x] ~~README.md~~ (N/A: not project-level)

**`docs/data-entry.md`** — new Authorization section: the bimodal policy table,
a worked `data-entry.yaml` + `acl.yaml` example, the breaking-change callout for
adopting a first `acl.yaml`, the view carve-out, and a `permission:` row in the
field table. Plus the `available_on` callout (RR-0LDY3W) stating it is not an
authorization boundary, cross-linked to acl-security.md.

**`docs/acl-security.md`** — new `command:*` section alongside `history:read`,
covering the named-permission mechanism, all three ACL modes, and the "What a
command permission actually confers" table (RR-37AYC0) stating plainly that
payloads are **not** read-gate scoped in any context.

## Accuracy corrections

Two places where existing text was **wrong**, not merely incomplete:

1. `e2e/tests/read-only-mode.spec.ts` claimed deferred phase-2 sites "remain
visible and 403 at the server on click". False for command buttons on both
halves — they did not 403, they *ran*. Corrected with the reason and a pointer
to the Go coverage.
2. The first draft of `docs/acl-security.md` (written earlier in this ticket)
implied view was uniquely wide-blast-radius. Corrected during code review — the
difference is degree, not kind.

## External Documentation

- [x] ~~API reference~~ (N/A: no new HTTP endpoint; `permission:` is config, and the 403 shape is standard)
- [x] ~~Migration guide~~ — covered inline by the breaking-change callout in `docs/data-entry.md` and the migration note in DEC-EIHQSU
- [x] ~~Changelog~~ (N/A: not maintained in-tree)

**Release-note material** (for whoever cuts the release): adopting a first
`acl.yaml` now requires `permission:` keys and grants on every command that
should keep working, and disables view commands entirely until TKT-2FDTJE.
