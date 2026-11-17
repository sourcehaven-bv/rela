---
id: DOCS-M61UORB
type: docs-checklist
title: 'Docs: Configure: edit flows.yaml triggers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not describe the Configure space)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

Updated `.claude/rules/configedit.md` (flows.yaml triggers, ordering, ACL name
check and its limits). Updated docs-project guides data-entry (Configure space:
three files, flows.yaml locked keys, ordering, ACL checks, read-all caveat),
acl-security (config:edit section), cli-reference (`--config-editing`) and flows
(moved triggers are edited in Configure), and regenerated `docs/`.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PRs)
- [x] API docs updated (if applicable)
