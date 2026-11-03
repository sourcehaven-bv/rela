---
id: DOCS-7ZNOUOW
type: docs-checklist
title: 'Docs: TKT-YK30D1'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: hint contract (`HintSource` gap once positioned), relay attachment order and fallback, retire-time flush order, journal rotation and retry, why unprefixed NOTIFY payloads are dropped
- [x] Function/type docs if public API: godoc on `FollowChannels`, `Local.Flush`/`Close`/`Stop`/`MarkAll`, `HintSender`, `HintSource`, `pgstore.Notify`/`Listen`, `filechannels.HintFeed`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing multi-writer change feed pattern in `.claude/rules/storage.md`)
- [x] Help text accurate (if CLI changes): `rela channel watch --poll` help says the poll is a recheck and other processes' changes usually arrive sooner

## User-facing Documentation

- [x] `GUIDE-realtime.md`: new "Changes from other processes" section (backends, delays, loss and 5 s recheck limit, same-user journal), updated live-update, agents, watch, inbox and busy-board text; `GUIDE-mcp-server.md` watch timing; `GUIDE-postgres-backend.md` `rela_channels` bullet with the cross-tenant caveat (TKT-147C19D) and the forged-hint load note (TKT-PMNFOGP); `GUIDE-cli-reference.md` `--poll`; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; the replay runs as each subscriber, and the cross-tenant visibility is documented in the postgres guide)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change; the SSE frames are the existing ones)
