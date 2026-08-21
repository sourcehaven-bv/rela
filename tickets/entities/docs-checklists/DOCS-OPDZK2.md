---
id: DOCS-OPDZK2
type: docs-checklist
title: 'Docs: Adopt commentlint in CI: comment-discipline gate + advisory report'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols documented — N/A in this repo (no Go code added); the
upstream tool's new code carries package-level docs explaining why each rule
exists and what it deliberately does not do
- [x] Non-obvious decisions carry a WHY — `.commentlint.yml` documents the
reason each disabled rule is disabled, with the evidence, rather than just
listing `false`
- [x] `CLAUDE.md` updated — commentlint documented alongside plimsoll and
arch-lint, covering the three high-signal rules and the suppression mechanisms

## Project Documentation

- [x] `CLAUDE.md` — new "Comment discipline" section under Lint
- [x] `.commentlint.yml` — inline rationale for every rule decision
- [x] `tickets/templates/entities/review-checklist.md` — gate checkbox plus
suppression guidance for future PRs
- [x] ~~`docs/` guides~~ (N/A: contributor tooling, not a user-facing feature.
No CLI command, no API surface, no UI. `docs/` is generated from the
docs-project rela graph and describes rela to its *users*; a CI linter for
rela's own source has no place there — the same reason plimsoll and arch-lint
are documented in `CLAUDE.md` only.)
- [x] ~~`README.md`~~ (N/A: same reasoning; README is user-facing)

## External Documentation

- [x] Tool README documents the new rules — the upstream repo covers
`doclink`, `param-contract`, `nil-contract` and `duplication`, each with its
rationale, its known false-positive classes, and a "Why not length" section
recording why the original `too-long` rule was abandoned
- [x] Suppression documented — README "Suppressing false positives" covers
both the inline directive and `.commentlint.yml`, and states that a reason is
required either way
- [x] ~~Migration notes~~ (N/A: additive change, nothing to migrate)

## Verification

- [x] Every documented command was run and produces the documented output
- [x] Counts quoted in docs match reality — re-verified after the v0.2.1 fix:
restatement 19, param-contract 5, doclink 58, nil-contract 100, duplication 119
(301 total)
