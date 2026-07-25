---
id: REV-T4E8HD
type: review-checklist
title: 'Review: visibility.Unrestricted'
status: done
---

## Automated checks

- [x] build, `just lint` (0 issues), `just arch-lint`, `just plimsoll` all pass
- [x] Affected suites pass; only `docscapture` fails, for the pre-existing local `frontend/dist/` reason

## Code review

- [x] `/code-review` run via cranky-code-reviewer.
- [x] The critical finding (typed nil in interface) was **independently verified with a probe** before acting, because it contradicted my own godoc. The probe confirmed the deny guard is bypassed and the read panics.
- [x] All three findings addressed: RR-5QQL1Z (critical), RR-65GNYB, RR-HHP90D.
- [x] Reviewer's arch-lint question answered: widening `docs`/`appbuildtest` is correct — a partial grep is a grep you cannot trust, which is the exact failure mode this ticket exists to fix.
- [x] No open critical or significant review responses.

## Verification

- [x] The grep claim is now true: `grep -rn "visibility.Unrestricted"` returns all six ungated sites.
- [x] Pass-through pinned behaviorally, not by pointer identity (memstore returns defensive copies, so identity was the wrong property).
- [x] Method-set enumerated by name rather than type-asserting `store.Store` — the latter only fires if the whole store is re-embedded and stays silent when a single write method is added.
- [x] PR: https://github.com/sourcehaven-bv/rela/pull/1208
