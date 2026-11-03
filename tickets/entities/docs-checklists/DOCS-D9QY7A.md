---
id: DOCS-D9QY7A
type: docs-checklist
title: 'Docs: Realtime 3/6: channel MCP tools, watch and CLI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the watch design (`internal/mcp/tools_watch_channels.go`, `internal/cli/channel_watch.go`), the run-as rules and the asserted_by lookup per build (`internal/cli/identity.go`), the terminal sanitizing in `printable`, and the reserved identity registry (`internal/principal`)
- [x] Function/type docs if public API: godoc on `Service.Read`, `ReadRequest`, `ReadView`, `UserText`, `Principal.AssertedBy`, `principal.IsRelaIdentity`, `principal.ValidSystemName` and `audit.AssertedByFrom`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the features are documented in the guides below)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention; the archguard test enforces the asserted_by rule itself)
- [x] Help text accurate (if CLI changes): kong help for `rela channel` and the global `--run-as` / `RELA_RUN_AS`, checked by the command-tree test

## User-facing Documentation

- [x] `GUIDE-cli-reference.md`: `rela channel` commands and the global `--run-as` / `RELA_RUN_AS`; regenerated into `docs/cli-reference.md`
- [x] `GUIDE-realtime.md`: MCP channel tools, watch, the CLI and where asserted_by is recorded; regenerated into `docs/realtime.md`
- [x] `GUIDE-acl-security.md`: run-as is attribution, not authentication; the self-asserted marker; job identities in acl.yaml; rela-server refuses it; regenerated into `docs/acl-security.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): the MCP tool descriptions and per-type JSON schemas describe the channel tools
