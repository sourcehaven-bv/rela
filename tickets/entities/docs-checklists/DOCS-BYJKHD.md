---
id: DOCS-BYJKHD
type: docs-checklist
title: 'Docs: Live views 2/3: per-subscription SSE frames via poke and gated re-read'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (liveHub and liveSub docs: fixed round clock, PUT-time baseline, cap rotation, fallback; liveReplayChain kept in step with the router)
- [x] Function/type docs if public API (useLiveSubscription and the manager exports; live.ts path builders)

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-facing setup change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting rule; the ACL reasoning is in docs/acl-security.md)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (docs/data-entry/api-reference.md: frames table and "Live views" section; docs/acl-security.md: live views paragraph, overload timing residual, claims-at-stream-open residual)
