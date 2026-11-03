---
id: DOCS-8YA4RU
type: docs-checklist
title: 'Docs: feed subscriptions (TKT-RUO1GY)'
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (seen-set and rebaseline, redirect policy, error classification, block values in ingest)
- [x] Function/type docs if public API (subscription package doc, Subscription, Run, Report, Fetcher, ingestspec.Mapping.Validate, ingest.MatchValues, scheduler.LoadConfig)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule)
- [x] Help text accurate (if CLI changes) (rela validate lists subscriptions; rela scheduler names both files when there are no tasks)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (new docs/feed-subscriptions.md; pointers from docs/webhooks.md and GUIDE-scheduled-tasks / docs/scheduled-tasks.md)
