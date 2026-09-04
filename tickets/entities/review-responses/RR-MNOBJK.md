---
id: RR-MNOBJK
type: review-response
title: storetest.RunAll is unconditional, so PR-B cannot keep postgres CI green as planned
finding: storetest.RunAll (storetest.go:150-170) is a flat unconditional list; the only gate is the Capabilities struct. pgstore/conformance_test.go:13 calls it identically to fs/mem, and the postgres CI job sets RELA_TEST_DATABASE_REQUIRED=1 so it cannot silently skip. Adding RunWorldTests in PR-B therefore runs it against pgstore too, forcing the suite to accept two contradictory outcomes (correct rows for fs/mem, rejection for pg) — which defeats the purpose of a conformance suite and loses the §14 conformance-before-second-backend discipline.
severity: significant
status: addressed
resolution: "Architect decision 2026-08-20: ACCEPTED. Resolution: RunWorldTests is gated by a Capabilities flag (Capabilities.Worlds) that fs/mem set in PR-B and pgstore sets in PR-C - the same one-commit-window discipline used for Capabilities.States in DOFYR1, including the TODO marker naming PR-C and deletion of the flag in PR-C. This preserves the S14 conformance-before-second-backend discipline without asking one suite to accept two contradictory outcomes. pgstore's loud rejection of a non-zero World (Q8) is asserted by a separate pg-specific test, not by the shared conformance suite."
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

The plan's PR-B mitigation ("pgstore rejects a non-zero World loudly, with the
postgres CI job asserting the rejection") does not survive contact with the
shared harness. The plan's risk table calls out the pg rejection but never
notices that `RunAll` is what forces the contradiction.

Verified: `internal/store/storetest/storetest.go:150-170` is flat and
unconditional apart from `Capabilities` (today just `Attachments`) and the
nil-factory checks for search. All three backends call it identically
(`fsstore/conformance_test.go:52`, `memstore/conformance_test.go:42`,
`pgstore/conformance_test.go:13`), and `ci.yml` sets
`RELA_TEST_DATABASE_REQUIRED: "1"` precisely so the pg suite cannot skip.

**Resolution:** this is the mechanism TKT-DOFYR1 already used, and the plan
should name it explicitly rather than leaving it to be improvised mid-PR:

1. Extend `storetest.Capabilities` with `Worlds bool` in PR-B; gate
`RunWorldTests` on it — the same pattern `Attachments` exists for. fs/mem set it
true in PR-B; pgstore flips it true in PR-C, and PR-C REMOVES the flag so it
cannot become a permanent opt-out (the DOFYR1 precedent: the flag exists for
exactly one commit window).
2. Add a small UNCONDITIONAL `RunWorldRejectionTests` in PR-B asserting a
backend without the capability returns a specific sentinel rather than a wrong
answer. That is the loud-rejection assertion and it belongs in the always-on
suite while the behavioral suite stays gated.
3. Pin the error precedence: AC9 (`AllStates` + `World` → `ErrInvalidQuery`)
can be unconditional in PR-B only if pgstore's invalid-query check runs BEFORE
its unsupported-world rejection, or the two sentinels collide.
