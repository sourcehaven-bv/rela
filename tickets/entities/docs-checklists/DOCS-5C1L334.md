---
id: DOCS-5C1L334
type: docs-checklist
title: 'Docs: TKT-EBARYBI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: design and ACL reasoning in the `internal/entitymanager/relationrules.go` doc block, why the check decides before writing (fs/mem do not roll back), transition-only semantics in `writerule.Set.CheckChange`
- [x] Function/type docs if public API: godoc on `writerule.Hop`, `writerule.Edges`, `CheckChange`, `entitymanager.ChecksRelation`, `entitymanager.WithPendingRelations`; metamodel `Enforce` and `validateValidationEnforce` comments updated; comment-lint gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-metamodel.md`: new "Rules that read relations" subsection (allowed shape, transition semantics, checked and not-checked writes, combined PATCH, ACL bit, cost); refused list updated; regenerated with `just docs`
- [x] `GUIDE-acl-security.md`: the accepted one-bit channel recorded under "What still leaks"

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the 422 shape is unchanged)
