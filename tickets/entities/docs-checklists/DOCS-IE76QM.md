---
id: DOCS-IE76QM
type: docs-checklist
title: 'Docs: Realtime 3/5: entity-change triggers and auto lifecycle'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the round algorithm, queue and retry rules (`internal/realtime/trigger.go`), the seen-set layout and recovery (`internal/realtime/seen.go`), auto rule ordering (`internal/realtime/trigger_rules.go`), and the held leader lock with its liveness check (`internal/appbuild/realtime_worker.go`)
- [x] Function/type docs if public API: godoc on `Triggers`, `TriggerDeps`, `EntitySource`, `KV`, `NewMessage.RequireOpen` and the worker parts; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the feature is documented in the realtime guide)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention; the worker follows the existing system-job rule with an AllowAllReader capability)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## User-facing Documentation

- [x] `GUIDE-realtime.md`: a "Triggers" section (rules, when_closed, delivery, retries, the single leader, the postgres pool size) and an `auto.when` section under "Lifecycle and periods" (once on change, a manual reopen stands, gates and conditions are skipped, write access to the entity is enough); regenerated into `docs/realtime.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): the OpenAPI document describes `author.system` and `triggered_by` on channel messages (`internal/openapi/channels.go`, updated goldens)
