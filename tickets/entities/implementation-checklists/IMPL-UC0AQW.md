---
id: IMPL-UC0AQW
type: implementation-checklist
title: 'Implementation: acl: migrate a scheduler read grant into an existing acl.yaml (new FileTypeACL)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Code implements the planned approach
- [x] Follows existing patterns (yaml.Node transform migration registered via `init()`, matching `short_id_default.go`)
- [x] No unplanned scope added
- [x] Edge cases handled

**Delivered:**

1. `principal.UserScheduler = "system:scheduler"` — fixed default identity for tasks with no `run_as`, replacing `principal.SystemUser()` (`$USER`).
2. `migration.FileTypeACL` + `acl-scheduler-grant` migration, wired into `projectsetup.getMigrateFiles`.
3. `migration.EnsureMapping` / `SetMapNode` in `yaml_util.go` — shared helpers, added while fixing RR-KG2FCX.
4. Docs: `docs-project/entities/guides/GUIDE-scheduled-tasks.md` (source) → regenerated `docs/scheduled-tasks.md`.

**Deliberately out of scope:** creating `acl.yaml` when absent (RR-SVQ5HE).

## Quality

- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK, no warnings
- [x] `just plimsoll` — clean
- [x] Tests written and passing
- [x] Coverage within floors (migration 84.1%, projectsetup 72.0%, scheduler 78.0%, principal 97.1%; default floor 50)
- [x] `just docs` regenerated; pre-push docs check passes

Only failing tests in the tree are the five `internal/docscapture` browser
tests, verified failing identically on a clean `develop` checkout (they need
Chrome + a built SPA).

## Verification

- [x] Behavior verified against acceptance criteria
- [x] Mutation-tested rather than assumed

**Mutation testing** (production code mutated, not fixtures — the failure mode
from earlier in this arc):

| Mutation | Result |
|---|---|
| Scheduler default reverted to `SystemUser()` | 3 tests fail |
| `acl.yaml` unwired from migrate file list | 2 tests fail |
| `EnsureMapping` stops repairing null in place | 6 subtests fail |
| `Detect` reverted to bare key-existence check | 5 tests fail |
| Dead-role repair removed | 2 tests fail |
| `Apply` postcondition removed | 1 test fails |

The postcondition was initially UNGUARDED — the first mutation run passed clean,
and a test was added specifically for it.

**Probes run before implementing** (recorded because they overturned the
ticket's premise):

- `scriptEntityReader(st, nil, nil).GetEntity` succeeds → policy-less projects have no regression.
- `stampTaskAuditContext(ctx, "nightly", "")` → `"deploy-bot"` (i.e. `$USER`), and `"unknown"` when `$USER` is unset.
