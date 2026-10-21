---
id: DOCS-HK5HYY
type: docs-checklist
title: 'Docs: End SSE streams when the verified token expires'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the expiry stays off `principal.Principal` (on `withCredentialExpiry`), the write-deadline cap (on `sseConn.expiry`), the early return in `handleSSE`, the leeway rule on `assertionLeeway`
- [x] Function/type docs if public API: `AssertionClaims.ExpiresAt`, `AssertedIdentity.ExpiresAt`, the Deprecated note on `JWTPrincipalResolver`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern; a context key set by a gate already exists)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-acl-security.md`: the live-views section says a stream behind the JWT gate ends when its assertion expires and the client reconnects; the revocation paragraph notes that streams follow the same bound; regenerated into `docs/acl-security.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no route or wire change; the stream ends like it does on a reload)
