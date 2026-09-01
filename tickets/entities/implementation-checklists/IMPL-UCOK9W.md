---
id: IMPL-UCOK9W
type: implementation-checklist
title: 'Implementation: Extract fileLayout (and mdCodec) off fsstore.FSStore (plimsoll ratchet 95 → ~81)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — the storetest conformance harness is the control, and changing it would destroy the evidence)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled (both steps landed; purity claim verified per-method before moving)
- [x] Error handling in place

## Test Quality

- [x] ~~Fixture builders~~ (N/A: zero test files changed — the right answer for this risk profile)
- [x] ~~No hardcoded values in assertions~~ (N/A)
- [x] ~~Only values that matter~~ (N/A)
- [x] ~~Interpolated values from objects~~ (N/A)
- [x] ~~Property comparisons from original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end (storetest conformance + fsstore suites, `-race`, all three build tags)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence (PR #1465, commits 672df120 + f64ebe4a):**
- FSStore 95 → **81**; the 14 removed are exactly the 14 moved methods.
Exported set diffed identical (33 vs 33) — the `store.Store` surface is
untouched.
- Both steps landed: `fileLayout` (6) and `mdCodec` (8).
- Purity is STRUCTURALLY enforced, not merely claimed: both new types use
value receivers (no pointer-receiver methods exist), and neither file references
entities/relations/propCache/observers/mu/txMu/echoes/watcher.
- Do-not-touch list verified: tx.go byte-identical; emit/notify* bodies
identical; lock-op count 66 in base and 66 on branch; notifyRenamed still emits
a single rename event; the mu-held-across-batch watcher property intact;
echo.go/gitcrypt.go/attachment.go/graphquery.go byte-identical.
- git-crypt inaccessible-shell path moved verbatim (still builds
len(props)+1 so IsLocked always fires); all 5 git-crypt tests pass.
- Gates verified at the real commit with `-count=1` in the correct
worktree: ./internal/store/... all pass, `-race` on fsstore, plimsoll,
arch-lint, comment-lint, go vet, and builds under default/memorybackend/
postgres tags.

## Quality

- [x] Code follows project patterns (urlHelpers-style focused immutable collaborator)
- [x] Checked for DRY opportunities (writeEntity/writeRelation pass-throughs noted and deliberately deferred — [[RR-Y9C2DP]])
- [x] No security issues introduced (path containment still in storage.RootedFS, moved by reference not reimplemented; git-crypt handling verbatim)
- [x] No silent failures
- [x] No debug code left behind
