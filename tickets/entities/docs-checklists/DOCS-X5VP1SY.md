---
id: DOCS-X5VP1SY
type: docs-checklist
title: 'Docs: TKT-J6GISC3'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious: retireID, retiredIDTaken, the HighestID SQL and the fs rewrite explain reservation, re-pointing and best effort
- [x] Function/type docs if public API: store.RetiredIDs, RenameResult.MentionsRewritten, entity.MentionRewrite and visibility.Resolver.MentionedBy/ResolveMentions are documented

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting pattern; the store capability is documented in its godoc)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): GUIDE-data-entry "Mentions after a rename" describes both backends; docs/data-entry.md regenerated
