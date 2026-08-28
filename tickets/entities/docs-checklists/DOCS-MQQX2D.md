---
id: DOCS-MQQX2D
type: docs-checklist
title: 'Docs: Reachability floor: merged-coverage pipeline + scupper (report-only baseline)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] ~~Function/type docs if public API~~ (N/A: no Go public API added)

The non-obvious parts are documented where the reader meets them:

- **`scripts/reachability.sh` header** — what a reachability floor is, and the
  distinction that carries the whole feature: *reached* ("executed at least
  once, by any test") is strictly weaker than *tested*. Without this stated up
  front the number invites being read as a quality score.
- **Why `-covermode=set`** — reachability is a boolean question; `set` answers
  exactly it, and all legs must share one mode or `covdata merge` rejects them.
- **Why `covdata merge`** — it unions counters, so "reached by ANY source" is
  the merge semantic rather than something the script has to implement.
- **Why graceful shutdown is in this change** — a `-cover` binary flushes
  counters only on a clean exit, so a killed server loses all e2e coverage.
- **Why some legs may fail without aborting** — coverage is still collected from
  the tests that ran; marked explicitly so it doesn't read as a swallowed error.

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing surface)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (see note)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

**Note on CLAUDE.md.** Its "Test Coverage" section documents the
`.testcoverage.yml` per-package floors, which this change does not alter. The
reachability floor is deliberately *not* written up there yet: while it is
report-only it imposes no contributor obligation, and documenting an advisory
number as a project convention would overstate it. The section should be
extended in the follow-up that turns on a threshold — at which point it does
create an obligation worth stating.

The `just reachability` recipe carries the same rationale in its comment, so a
contributor who finds the recipe gets the context without reading the script.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no user-visible behavior change)
- [x] ~~API docs updated (if applicable)~~ (N/A)

This adds a CI signal and a developer recipe. Nothing changes for someone
running or operating rela, so there is nothing to announce.
