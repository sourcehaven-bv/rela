---
id: REV-V19OGM
type: review-checklist
title: 'Review: output.Writer: delete dead/internal/single-caller surface — plimsoll directive deleted (23 → 14)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`-count=1` on cli/output/metamodel + full suite; all 25 CI checks green pre-fix, re-running post-fix)
- [x] Linters pass (golangci-lint 0 issues, plimsoll silent with the directive DELETED, arch-lint, comment-lint clean across 11066 comments)
- [x] Coverage floors hold (internal/output at 95.7% against a strict 90% floor; total 78.2%)

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, briefed on deletion/API-shrink failure modes rather than pure-move ones)
- [x] All critical/significant findings addressed: [[RR-UACSDM]] — the CLI exception rationale asserted "nothing reads these fields" while `runKong` reads all four globals 78 lines later. Corrected to the narrower true claim.
- [x] Minor/nit findings addressed: [[RR-H1R2R8]] (stale `SchemaEntityDef` doc reference; 13 unkeyed struct literals now keyed), [[RR-A0Q9O6]] (dropped already-discarded parameter, verified against base)

## Verification

- [x] **`WriteRelations` genuinely dead** — verified across all three build tags (default/memorybackend/postgres), cmd/, e2e/, and non-Go dispatch surfaces; no `MethodByName` exists anywhere in the repo, so no reflective dispatch could break
- [x] **Coverage did not silently go dark** — checked per-symbol, not by floor: `newBorderlessTable`, `writeSeparator`, `writeFooterSummary` all at 100%; all six moved schema methods at 100% in their new home; 6 of 8 deleted tests moved 1:1 with byte-identical assertions
- [x] **JSON byte-identical** — moved bodies diffed verbatim: same map keys, same optional-field branches, same `SetIndent("", "  ")`. No wire-visible break for CLI consumers
- [x] Directive genuinely deleted (Writer at 14, under the default 20) — plimsoll runs silent, not leaning on a leftover pin
- [x] Placement respects the consumer-side-interface rule: interfaces moved to their single consumer and became unexported; arch-lint clean

**Out-of-scope discovery filed separately as [[BUG-J3YJCC]]:** `-tags e2e` does
not compile on `develop` (`e2e_test.go` is three parameters behind `NewApp`).
Pre-existing and unrelated to this PR, but it means the e2e tag is already red
and cannot catch regressions.
