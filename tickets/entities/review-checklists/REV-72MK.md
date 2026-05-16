---
id: REV-72MK
type: review-checklist
title: 'Review: Lift workspace analysis methods to internal/analysis'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] No critical findings
- [x] 3 significant findings addressed (stale comments, drift-warning doc, slog on error masking)
- [x] 4 minor findings addressed (doc accuracy, AnalyzeOptions → Options rename, MetamodelAccessor moved to CLI)
- [x] 2 won't-fix (test pattern intentional; symlink-loop pre-existing)
- [x] 1 leverage already applied (compile-time bundle assertions)
- [x] Tests pass under `-race`
- [x] `just ci` green

## Disposition

See IMPL-10FT for the full table.

**Highlights:**

- **Drift documented.** `cliAnalyze` now spans three subsystems (analysis + attachment + renametype) because attach/list/rename subcommands piggyback the bundle. Added an explicit drift-warning comment + follow-up note rather than splitting the bundle in this PR. The split is a separate ticket.
- **Silent errors surfaced.** `FindOrphansWithScope` and `collectEntities` previously swallowed tracer/iterator errors silently. Now logged via `slog.Warn`. Returning errors from those methods would ripple through 4 facade methods + AnalyzeAll + CLI callers — deferred as a follow-up.
- **MetamodelAccessor relocated.** Interface had exactly one consumer (`validate.go`); it now lives there as private `metamodelAccessor`. analysis package no longer carries a type it doesn't consume — CLAUDE.md "interfaces at the call site."
- **AnalyzeOptions → Options.** Finished the no-stutter rename pass that started with `AnalysisSummary → Summary` (revive).
