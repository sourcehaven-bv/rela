---
id: DOCS-KICFJWI
type: docs-checklist
title: 'Docs: TKT-W4F7VCB'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: godoc on `acl.Policy.FieldRestricted` states when a property is restricted and why an unconditionally listed one is not; the agreement test explains why it covers live reads only
- [x] Function/type docs if public API: godoc updated; comment-lint gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] Help text accurate (if CLI changes): the flow guard and template load errors say when the read is allowed

## User-facing Documentation

- [x] `GUIDE-flows.md` (Conditions: what restricted means, with an acl.yaml and flows.yaml example) and `GUIDE-realtime.md` (the restricted-property rule); regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: the rule is documented where operators write the config)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated~~ (N/A: no API change)
