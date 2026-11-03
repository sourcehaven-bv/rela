---
id: DOCS-NAZORH
type: docs-checklist
title: 'Docs: Realtime 3/3: channel service, HTTP API and live delivery'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the guard's delete, detach and restore rules (`internal/appbuild/channelguard.go`), the type record merge rules (`internal/channels/typerecord.go`), and the reload check (`internal/realtimecfg/reload.go`)
- [x] Function/type docs if public API: package docs on `internal/realtime` (service.go) and `internal/channels` (channels.go); godoc on `ChannelService`, `channelsHandler` and the route parser in `internal/dataentry/channels_handler.go`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the feature is documented in the realtime guide)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention; the guard follows the existing consumer-side interface and visibility rules)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: the only CLI change is the anchor-blocked error message, covered by `internal/cli/anchor_blocked_test.go`)

## User-facing Documentation

- [x] `GUIDE-realtime.md`: sections "HTTP API" (routes under `_channels`), "Errors" and "Live updates"; regenerated into `docs/realtime.md`
- [x] `GUIDE-acl-security.md`: section "Realtime channel routes"; regenerated into `docs/acl-security.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): the OpenAPI document describes the channel routes (`internal/openapi/channels.go`, updated goldens)
