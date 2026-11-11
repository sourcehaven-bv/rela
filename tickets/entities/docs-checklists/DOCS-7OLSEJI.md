---
id: DOCS-7OLSEJI
type: docs-checklist
title: 'Docs: TKT-XI7IDBS'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the ceiling applies after the role union and only removes, why an empty run state is the initial state, why a stored declaration skips `outside_edits:`, why a rename is refused while a run is stored, why the gate is asked again inside the write Tx, why redaction-only reads skip the run lookup
- [x] Function/type docs if public API: godoc on `flowedits.New`, `WithinRun`, `affordances.FlowEditCeiling`, `appbuild.FlowEdits`, `flowcfg.OutsideEdits`, `flowrun.Query.SubjectIDs`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing field-ceiling and collection-read rules; no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-flows.md`: `outside_edits` row in the keys table and a section "Edits outside a flow" (rules, precedence, runs, own-writes exemption, surfaces that hold and do not, renames, lookup failure, non-atomic check, per-process run store, stuck runs, file rules, merge conflicts); regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: the ceiling only narrows existing field gates; no change to the ACL model)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): `apiwire` godoc for `Transition.Reason` names `read-only`, which feeds the OpenAPI description; the SPA type comment matches
