---
id: RR-W771AR
type: review-response
title: TestChoices_PoCFlow is not the PoC flow, and the real stale path has no test
finding: 'internal/realtime/session_test.go:906-916 removes in: from the answer type so that the stale click reaches session_moved. The PoC and the docs example both keep in: [asking], so a real stale click returns 409 session_wrong_state. The test comment still says ''in the PoC session''. No SPA test covers onChoose receiving session_wrong_state (useSessionDecisions.ts:420 maps it to stale and refetch). Fix: keep the modified case and rename its comment, add a case with the unmodified PoC config that asserts TypeSessionWrongState on the second click, and add a useSessionDecisions test in which onChoose gets problem(''session_wrong_state'') and the test asserts the stale toast, refetch and no composer.select.'
severity: minor
resolution: 'TestChoices_PoCFlow is now a table: the unchanged PoC config asserts 409 session_wrong_state on the stale second click, and the case with in: removed from the answer type asserts session_moved. The comment describes both. Added a useSessionDecisions test in which onChoose gets session_wrong_state, and it asserts the stale toast, one refetch, no handle, no composer.select and no focus.'
status: addressed
---
