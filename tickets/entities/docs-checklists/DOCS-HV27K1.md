---
id: DOCS-HV27K1
type: docs-checklist
title: 'Docs: related() in views, next-action, CLI filter, validation, automation, state machine and ACL when:'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc (`relresolve.NewStoreBinder`, `statemachine.Set.TraversingWhens`, `affordances.PrimeTraversals`, `queryplan` condition index derivation)
- [x] Non-obvious decisions explained in comments (the shared memo key in `affordances/traversal.go`: a miss only denies; the face refusal in `validation.ruleTraversals`)
- [x] ~~Package docs updated if package purpose changed~~ (N/A: no package changed purpose)

## Project Documentation

- [x] CLAUDE.md updated with new patterns (packages table gains `internal/relresolve`)
- [x] docs/ updated for changed behaviour (GUIDE-metamodel "Where `related(...)` works" with the surface/gate table; GUIDE-data-entry list and next-action conditions and the form refusal; GUIDE-acl-security "`related(...)` in a grant's `when:`"; regenerated docs/)
- [x] ~~Architecture docs updated~~ (N/A: no package boundary change; arch-lint clean)

## External Documentation

- [x] ~~README updated~~ (N/A: the guides cover the feature)
- [x] CLI reference updated (GUIDE-cli-reference: `--filter related()` examples and the validations note)
- [x] ~~API docs updated~~ (N/A: no HTTP/MCP wire change; a refused traversal surfaces through the existing error body)
