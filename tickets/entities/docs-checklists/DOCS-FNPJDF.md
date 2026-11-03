---
id: DOCS-FNPJDF
type: docs-checklist
title: 'Docs: Realtime 4: channel lifecycle, retention, actions and promote'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: tombstone filing and confirmation (`internal/appbuild/channelguard.go`), the sweep and purge rules (`internal/realtime/sweep.go`, `retention.go`), the deletion CAS (`internal/channels/channels.go`), the promote flow and its revert rules per backend (`internal/realtime/promote.go`, `internal/appbuild/realtime_promote.go`), and `entitymanager.CreateLinked`
- [x] Function/type docs if public API: godoc on `Service.Lifecycle`, `Service.RunAction`, `Service.ReleaseAction`, `Promoter`, `PromoteRequest` (with the pinned triggered_by format), `ClassifyCreateError` and the new channel store methods; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the feature is documented in the realtime guide)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention; the channel store joins the store Tx through the existing filer pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## User-facing Documentation

- [x] `GUIDE-realtime.md`: sections "When an entity is deleted" (block, detach, delete and retention), "Actions and promote" (atomicity per backend, warnings, release), "Lifecycle and periods" and the "HTTP API" and "Errors" tables; regenerated into `docs/realtime.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): the OpenAPI document describes the lifecycle, action and release routes (`internal/openapi/channels.go`, updated goldens)
