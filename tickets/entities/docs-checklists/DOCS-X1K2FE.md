---
id: DOCS-X1K2FE
type: docs-checklist
title: 'Docs: ACL read-side (PR 2/2): list endpoints + sidebar counts + pagination headers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — scopedSortedEntities ACL ordering contract, errACLListQuery/errListLoad sentinels, sidebarCounts single-mode rationale (RR-2O27/RR-BZ4M), readableSubset batching, noCacheMiddleware Vary, Policy.Validate write⊆read scope
- [x] Function/type docs if public API — `App.SetPrincipalHeader`, `Policy.Validate` godoc extended

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level changes)
- [x] ~~CLAUDE.md updated~~ (N/A: no new patterns — consumer-side interface + capability rules already cover readGate.ReadQuery)
- [x] ~~Help text accurate~~ (N/A: no CLI flag changes — --principal-header text unchanged)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: project has no changelog file; release notes derive from PR titles)
- [x] API docs updated — `GUIDE-acl-security` read-path rewrite (both gates, search-after-ACL contract, write⊆read invariant + scope, caching/Vary, sidebar menu decision, config-filter perf caveat, _position status); `docs/acl-security.md` regenerated via `just docs`; `docs/security.md` policy-mode row + write⊆read semantics bullet
