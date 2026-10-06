---
id: DOCS-TDLNNN
type: docs-checklist
title: 'Docs: entity.Ref and one gated resolver in internal/visibility'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the resolver (`internal/visibility`)
documents the head/content-tail rule, and `edgeowner.go` the face that owns a
content edge
- [x] Function/type docs if public API: `entity.Ref`, `visibility.Resolver`,
`EndpointsReadable` and `EndpointsReadableErr` carry godoc

## Project Documentation

- [x] ~~README updated~~ (N/A: no change to setup or the feature list)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; the zero-face rule came
with TKT-YJ17N2)
- [x] Help text accurate: history, restore and history-purge take an
`ID@face` address (`docs/cli-reference.md`, #1724)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes
come from PR titles)
- [x] API docs updated: `docs/data-entry/api-reference.md` (per-face
attachments, export and history), `docs/content-states.md` (per-face reads and
sub-resources), `docs/acl-security.md` (face gate on reads, command payloads),
`docs/mcp-server.md`, `docs/postgres-backend.md` (purge per face),
`docs/data-entry.md`
