---
id: DOCS-SYNCR8
type: docs-checklist
title: 'Documentation: sync is a client of the authorized API (fancy-browser)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc/comments describe the new model: splice.go (the `_redacted` three-state splice + no-data-loss rationale), state.go (two-token Baseline), schema.go (handshake + shape check), push.go (temp-id adoption + relation remap), pull.go (feed-404 guard), relation_read_handler.go (dual-endpoint gate + fail-closed meta)
- [x] `isSyncExemptV1Path` + the tightened `isCSRFExempt` Origin check documented in middleware_security.go, cross-referenced from `nonBrowserExemptPrefixes`
- [x] Retirement documented where the old handlers were (sync_handler.go, sync.go comments explain the manifest is all that remains)
- [x] ~~README / package-level docs~~ (N/A: no package README; the model is documented in acl-security.md, below)

## Project Documentation

- [x] `docs/acl-security.md` "Sync is a client of the authorized API (fancy browser)" section rewritten (reads via /api/v1 inherit redaction; pull-apply splice via `_redacted`; push via /api/v1 field-write ACL; ETag opaque; feed-404 skip; Mode A vs deferred Mode B)
- [x] `docs-project/entities/guides/GUIDE-acl-security.md` mirror updated identically (source of truth for the generated doc)
- [x] Lead-in references at the two earlier mentions of sync in acl-security.md corrected (inherits redaction via /api/v1, not a private redacted channel)
- [x] ~~External / user-facing docs (docs-project guides beyond acl-security)~~ (N/A: sync is an operator/CLI machine-to-machine surface; the security guide is the relevant doc surface and is updated)
