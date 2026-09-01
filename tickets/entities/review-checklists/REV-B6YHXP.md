---
id: REV-B6YHXP
type: review-checklist
title: 'Review: Extract fileLayout (and mdCodec) off fsstore.FSStore (plimsoll ratchet 95 → ~81)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — verified at the real commit with `-count=1` in the correct worktree: `./internal/store/...` (storetest conformance harness + fsstore + all sibling backends), `-race` on fsstore, builds under default/memorybackend/postgres tags
- [x] Linters pass (plimsoll at 81, arch-lint OK, comment-lint clean, go vet clean, golangci-lint)
- [x] Coverage floors hold

## Code Review

- [x] `/code-review` run (cranky-code-reviewer, independent verification of the 11c02c1b..672df120 diff)
- [x] All critical/significant findings addressed (none — this was the highest-risk PR of the arc and it came back clean)
- [x] Minor finding addressed: [[RR-0QSEHW]] (stale `s.schemas` comment — fixed in f64ebe4a); nit deferred with reason: [[RR-Y9C2DP]]

## Verification

- [x] Do-not-touch list verified item by item: tx.go byte-identical; emit/notifyPut/notifyDelete/notifyRenamed bodies identical; lock-op count 66 in base and 66 on branch (no second lock introduced); notifyRenamed still emits a single rename event, never delete+put; the mu-held-across-batch watcher property intact; echo.go/gitcrypt.go/attachment.go/graphquery.go byte-identical
- [x] Purity claim verified as STRUCTURALLY enforced, not merely asserted: both new types use value receivers (no pointer-receiver methods exist, so they cannot mutate), and neither file references entities/relations/propCache/observers/mu/txMu/echoes/watcher. The `schemas` map aliasing is unchanged from base
- [x] `store.Store` surface unchanged: exported method sets diffed identical (33 vs 33)
- [x] Method count verified at 81; the 14 removed are exactly the 14 moved
- [x] git-crypt inaccessible-shell path moved verbatim (still builds len(props)+1 so IsLocked always fires); all git-crypt tests pass
- [x] Zero test files changed — correct for this risk profile, since the conformance suite is the control

**Process note carried into later arc steps (recorded on [[RR-Y9C2DP]]):**
verify in the worktree that actually has the commit checked out and use `go test
-count=1`. A cached PASS from the base commit is indistinguishable from a real
one — the reviewer hit exactly this and caught it.
