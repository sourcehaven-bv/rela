---
id: DOCS-LSFNI8
type: docs-checklist
title: 'Docs: background jobs seam + retry/deadline contract (TKT-YOED3R)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package doc explains the seam, the tiering, and both invariants
(`internal/jobs/jobs.go`)
- [x] Every exported symbol documented (`Queue`, `Job`, `Retry`, `Handler`,
`Collector`, `WithDeferral`, `NewMemoryQueue`, `NewPostgresQueue`)
- [x] Nil contracts stated where they matter (constructors reject nil
collaborators; `Services.Jobs()` is never nil)
- [x] Non-obvious upstream behaviour documented at the site that depends on it
— `backendRetryBudget` carries the full reasoning for why neoq is handed an
unreachable budget and no deadline, since that code reads as wrong without it

## Project Documentation

- [x] `CLAUDE.md` — new architecture rule covering the two invariants (retry
enum stays vague; jobs never run before their transaction closes) plus the
`internal/jobs` package-table entry
- [x] `docs/background-jobs.md` — new user-facing page: deployment tiering,
retry vocabulary, cadence/deadline interaction, write ordering,
duplicate-submission semantics, the 15-minute handler cap, and an honest "what
this does not do" section
- [x] ~~`docs/cli-reference.md`~~ (N/A: no new commands)
- [x] ~~`docs/metamodel.md`~~ (N/A: no schema change)
- [x] ~~`README.md`~~ (N/A: internal subsystem, not a project-level change)

## Accuracy

- [x] Docs updated to match the post-review implementation, not the original
design. Specifically: the duplicate-submission section and the handler timeout
were added *after* review found that payload-hash dedup and a silent 30s cap
were real behaviours — documenting the design as first written would have been
wrong.
- [x] Ephemeral-on-exit is described as intended behaviour with the reasoning,
not as a limitation, so it does not read as a bug to a future reader
- [x] `just lint-md` — 0 issues across 253 files

## Deliberately not documented

- The `__rela_*` payload keys are an implementation detail of the neoq adapter
and are stripped before a handler sees a payload. Documenting them would invite
callers to depend on them.
- Exact retry counts and backoff timings. These live in `retry.go` and are
meant to be retunable without a docs change — publishing the numbers would
recreate the coupling the flat enum exists to prevent. The docs describe intent
("a few times", "roughly two days") instead.
