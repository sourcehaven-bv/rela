---
id: DOCS-54NM44
type: docs-checklist
title: 'Docs: Query-driven entity lists in sidebar navigation groups'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (SidebarEntityQuery held-content rule, useSidebarEmptyGroups shown state, Sidebar load generation and stale-response guard, EffectiveNavSort)
- [x] Function/type docs if public API (NavEntitiesEntries, EffectiveNavSort, SidebarEntities wire type, NavigationEntry fields in config.ts)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not describe navigation config)
- [x] CLAUDE.md updated (if new patterns) (root CLAUDE.md: the menu carries no per-principal data; `entities:` rows come from the ACL-gated list API)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] API docs updated (if applicable) (GUIDE-data-entry "Entity lists in a group"; GUIDE-acl-security sidebar paragraph; GUIDE-metamodel default scope row; docs/ regenerated with `just docs`)
