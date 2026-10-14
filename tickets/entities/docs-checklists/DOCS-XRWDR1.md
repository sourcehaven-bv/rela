---
id: DOCS-XRWDR1
type: docs-checklist
title: 'Docs: Reload schedules and subscriptions without a restart'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (reloadConfig, maybePrune prune guard)
- [x] Function/type docs if public API (WatchConfig, StartBackground godoc)

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not cover the scheduler config lifecycle)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI flag or help change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog; release notes come from PR titles)
- [x] API docs updated (docs/scheduled-tasks.md and GUIDE-scheduled-tasks gain "Config changes"; docs/feed-subscriptions.md drops the restart note)
