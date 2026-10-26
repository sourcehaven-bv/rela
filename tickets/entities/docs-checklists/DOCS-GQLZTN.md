---
id: DOCS-GQLZTN
type: docs-checklist
title: 'Docs: Sessions 4/5b: timer and trigger outputs (run_as)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: job vs person marks and the writer rule (`internal/realtime/outputs.go`), the grant check's face handling (`internal/realtimecfg/grants.go`), the file-tier crash repair (`internal/channels/filechannels`)
- [x] Function/type docs if public API: godoc on `acl.JobGrants`, `principal.ValidJobIdentity`, `channels.OutputMark.Job`/`Timer`, `realtimecfg.Session.RunAs`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela validate` runs the realtime load whenever `realtime.yaml` exists; `rela channel` renders `run_as_changed`

## User-facing Documentation

- [x] `GUIDE-realtime.md`: "Timers and triggers" outputs, `session.run_as`, template rules for job moves, load checks, loops, reload behaviour, crash notes; regenerated into `docs/realtime.md`
- [x] `GUIDE-acl-security.md`: "Session `run_as` identities"; `GUIDE-mcp-server.md`: `run_as` and the timer `outputs` count on `read_channel`; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI result code `run_as_changed` and the timer `outputs` count
