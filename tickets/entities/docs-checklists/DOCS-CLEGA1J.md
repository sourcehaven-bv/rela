---
id: DOCS-CLEGA1J
type: docs-checklist
title: 'Docs: TKT-FWQZGAM'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the worker's package comment (leader overlap bound, tick cap, round deadline, identity), why a create output and a backoff run on a detached context, the joint budget of worker pins and keyed slots, why `lead` returns on an ended context
- [x] Function/type docs if public API: godoc on `RunFlowWorker`, `DueRound`, `JoinRecheck`, `TryKeyedLock`, `CheckLockRoom`, `HeldLock`; comment-lint gate clean and no new comment-report findings

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: the worker follows the realtime worker's existing leader-lock pattern)
- [x] Help text accurate (if CLI changes): `rela flow start` says the run is retried while rela-server or the desktop app runs, and how to cancel it

## User-facing Documentation

- [x] `GUIDE-flows.md`: restored the #2198 text (timers fire and failed steps are retried while a server runs), the "flow worker" paragraph (interval, hosts, one leader, tenants, per-pass cap, `pool_max_conns` of 4), the 5-minute retest of `when:` joins; `GUIDE-realtime.md` pool minimum with flows; `GUIDE-mcp-server.md` retry wording; regenerated with `just docs`
- [x] Frontend: `FlowWaiting.vue` shows the next attempt again; en and nl i18n strings restored

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API shape change; `next_attempt` was already on the run view)
