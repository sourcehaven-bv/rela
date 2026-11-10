---
id: DOCS-ADEQTW4
type: docs-checklist
title: 'Docs: TKT-9IT3LN8'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the pending mark as "unsettled in this state", why a create claim is put back on failure, the backoff schedule, why a KeptEntityError counts as done, the orphan-header grace period
- [x] Function/type docs if public API: godoc on `Engine.Start`, `Cancel`, `CloseHeader`, `FlowOf`, `Due`, `FlowControl` and its methods, `Pending.Failures`, `audit.FlowStep.By`; Nil tags on the engine deps; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela flow start`, `rela flow runs` and `rela flow cancel` help and `GUIDE-cli-reference.md`

## User-facing Documentation

- [x] `GUIDE-flows.md`: status block, "Running flows" section (commands, permissions, steps, retry and backoff, orphan runs, per_subject, storage, upgrade note) and audit `by`; `GUIDE-audit-log.md`: the `by` field; regenerated with `just docs`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP API in M1d; the CLI is the only surface)
