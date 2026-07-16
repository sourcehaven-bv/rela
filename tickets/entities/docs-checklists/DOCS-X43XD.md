---
id: DOCS-X43XD
type: docs-checklist
title: 'Documentation: Multi-step (wizard) forms (TKT-CHLAJ)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — `useFormWizard` (getter-not-computed rationale, clamp, URL-sync echo), `DynamicForm` wizard branch (pruneWizardHidden reconciliation, affordance-vs-condition scope), condition engine reactivity note
- [x] Function/type docs if public API — `FormWizard` interface documented; Go `Form.Steps`/`FormStep`/`VisibleWhen`/`RequiredWhen` have godoc; `conditionlint` package doc explains the stricter-superset caveat

## Project Documentation

- [x] docs/data-entry.md updated — new "Multi-step (wizard) forms" section: `steps:`, `visible_when`/`required_when` grammar table + examples, navigation/URL, per-step validation, hidden-branch-not-saved, UX-only + server-authoritative note, and the `rela validate` author-time check
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting convention; follows existing composable/config patterns)
- [x] Help text accurate — `rela validate` now reports condition errors (no separate help text; surfaced inline)

## External Documentation

- [x] ~~Changelog entry~~ (N/A: project has no maintained changelog file)
- [x] ~~API docs~~ (N/A: the `/api/v1/_config` shape extends via existing config structs; no new endpoint)
