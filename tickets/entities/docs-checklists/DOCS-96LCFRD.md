---
id: DOCS-96LCFRD
type: docs-checklist
title: 'Docs: Erase deleted-entity state from automation job payloads'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: deletedStates (why sealed, why the generation), sweep constants (retention, first-sweep delay, batch size), pgQueue (own pool)
- [x] Function/type docs if public API: appbuild.PurgeDeletedJobStates, entitymanager.PurgeNotifier

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing notifier and state.KV patterns)
- [x] Help text accurate (if CLI changes): GUIDE-cli-reference notes that a committed history-purge erases the state of waiting on.deleted jobs

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): GUIDE-metamodel "Retention of the deleted state"; GUIDE-postgres-backend "Job table retention"; docs/ regenerated with just docs
