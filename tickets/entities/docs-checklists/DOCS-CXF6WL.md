---
id: DOCS-CXF6WL
type: docs-checklist
title: 'Docs: Roll-up properties'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README lists no property kinds)
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no new flags; show and list print roll-ups like other properties)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] API docs updated (if applicable)

Evidence: `docs-project/entities/guides/GUIDE-metamodel.md` gains a "Roll-up
properties" section (keys, computed vs roll-up, access, where values appear, the
`rollups=1` / `list_id` list opt-in, writes, caching), and `docs/metamodel.md`
is regenerated from it. CLAUDE.md lists `internal/rollup`.
