---
id: DOCS-GSKNOC
type: docs-checklist
title: 'Docs: classify the unreached set with reasoned coverage-ignore directives'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols documented
- [x] Non-obvious decisions explained in comments

No exported symbols were added or changed. The documentation this change adds
*is* the 260 directive reasons: each names a category and states, for its
specific call site, why the code cannot run under test. `--require-reason` makes
an unexplained dismissal a hard error, so the reasoning stays in the diff where
a reviewer can challenge it.

Four pre-existing doc comments that a directive had run into were re-separated
so each reads on its own; no existing prose was removed or reworded.

## Project Documentation

- [x] `COVERAGE-HONESTY-REPORT.md` regenerated against current code
- [x] Superseded tooling removed

**Report.** Rebuilt from a pipeline run against `origin/develop` (b34dbae8). The
2026-08-28 version was stale in every headline figure: it cited 291 directives,
75.4% coverage and 6,767 uncovered blocks measured on a 7-week-old tree, and
described the `scupper:ignore` dialect the landed pipeline does not read.

The rebuilt report states which legs ran (unit + cross-package) and which did
not (e2e, postgres), and flags `pgstore` (6.8%), `dataentry` (82.9%) and
`rela-server` (10.0%) as understated by the measurement rather than by the code.

The previous report's "gap-group" counts came from a 63-agent classifier sweep
that cannot be reproduced from the pipeline. They were removed rather than
carried forward unverified — a number nobody can regenerate is not an audit.

**Removed.** `scripts/coverage-generous.sh`, superseded by the
`scripts/reachability.sh` that landed with TKT-DWO4ZB: same covdata merge, same
`-covermode=set` rationale, same legs, and additionally wired into `just
reachability` and CI. Its one unique step built an instrumented `bin/rela` that
no test drives, contributing no coverage.
