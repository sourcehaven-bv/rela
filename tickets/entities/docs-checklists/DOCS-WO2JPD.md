---
id: DOCS-WO2JPD
type: docs-checklist
title: 'Docs: Sessions 4/5a: message-driven graph outputs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: mark lifecycle and crash windows (`internal/channels/outputs.go`), supersede and claim renewal (`internal/realtime/outputs.go`), retry identity (`internal/appbuild/realtime_outputs.go`)
- [x] Function/type docs if public API: godoc on `channels.OutputMark`, `OutputResult`, `CheckOutputStep`, `realtimecfg.Output`, `Template`, `acl.RetoolRequest`, `entitymanager.CheckRelate`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel` shows output results on notices and after a post

## User-facing Documentation

- [x] `GUIDE-realtime.md`: `outputs:` with `set:` and `create:`, templates, field grants, the 403 pre-check, retries and their identity, per-tier guarantees, `on_final: close`, notice `outputs` block; regenerated into `docs/realtime.md`
- [x] `GUIDE-mcp-server.md` and the audit log guide: outputs on `read_channel`/`post_message` and `triggered_by` for output writes; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI notice `outputs`, result kinds, `session_output_denied` and `outputs_pending`
