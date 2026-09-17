---
id: PLAN-YSHHA0
type: planning-checklist
title: 'Planning: classify the unreached set with reasoned coverage-ignore directives'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem is clearly defined
- [x] Existing code reviewed
- [x] Prior art identified

TKT-DWO4ZB landed the pipeline and explicitly deferred the classification. The
work here is that deferred half, preserved on a branch cut from a 2026-07-22
merge base and rebased onto a develop 316 commits ahead.

The decisive prior-art finding: TKT-DWO4ZB chose `coverage-ignore` as the
directive vocabulary and rejected `//scupper:ignore` as a redundant second
dialect. The preserved branch used the rejected spelling throughout.

## Approach

- [x] Approach chosen and justified
- [x] Alternatives considered

**Chosen.** Rebase, reattach each directive to the construct it classified,
convert the dialect, then validate every directive against a fresh merged
profile and drop the ones that no longer describe reality.

**Alternatives considered.**

1. *Land the `scupper:ignore` spelling and add `-d scupper:ignore` to the
   pipeline.* Rejected: it reverses a decision TKT-DWO4ZB made for a stated
   reason (`go-test-coverage` reads `coverage-ignore` from the same comments),
   and would leave the repo with two spellings for one concept.
2. *Keep `scripts/coverage-generous.sh` alongside `scripts/reachability.sh`.*
   Rejected — see Risk Assessment.
3. *Abandon the branch as too stale.* Rejected: 84 of 109 annotated files had
   changed upstream, but the constructs themselves mostly survived; 436 of 457
   directives reattached cleanly.

## Risk Assessment

- [x] Risks identified
- [x] Mitigations planned

**Risk: a rebased directive silently reattaches to unrelated code.** A comment
carries no binding to its subject, so a naive merge can strand one. Mitigated by
resolving each conflict against the upstream construct and dropping any
directive whose anchor no longer exists (21 dropped), then verifying the whole
set against a real coverage profile.

**Risk: a directive dismisses code that actually runs**, hiding a real gap.
Mitigated by the column-aware profile cross-check, which found and removed 3
such blocks.

**Risk: duplicate coverage scripts drift.** `scripts/coverage-generous.sh` is
the direct predecessor of the landed `scripts/reachability.sh`: same covdata
merge, same `-covermode=set` rationale, same legs. Its only unique step builds an
instrumented `bin/rela` that no test drives, so it contributes no coverage.
Keeping both would leave two scripts claiming the same job, one unwired to CI.
Mitigation: drop it.
