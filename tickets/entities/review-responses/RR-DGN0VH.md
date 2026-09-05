---
id: RR-DGN0VH
type: review-response
title: _originalData defineExpose test seam widens production API for one test
finding: resetCreateForm's ordering invariant (re-baseline originalData AFTER the awaited dry-run, so adoptLockedFieldValues is included) is pinned by exposing _originalData on defineExpose. Reviewer notes the same invariant is reachable without a seam by asserting the form is not dirty after a subsequent keystroke.
severity: nit
resolution: 'Wont-fix, deliberately. The suggested alternative was tried first and does NOT work: dirty is computed by JSON.stringify comparison that is key-PRESENCE sensitive, so an untouched field contributes no key and a type-and-revert round trip reports dirty for reasons unrelated to the invariant — the test failed against correct code. Asserting the baseline directly is the honest assertion. The seam is underscore-prefixed, documented as a test seam, read-only, and mutation-verified to catch the ordering bug it exists for.'
reason: 'The suggested seam-free alternative was implemented first and fails against CORRECT code: `dirty` compares JSON.stringify snapshots that are key-presence sensitive, so an untouched field contributes no key and a type-then-revert round trip reports dirty for reasons unrelated to the invariant under test. Asserting the re-baselined value directly is the only honest assertion available. The seam is underscore-prefixed, documented as a test seam, read-only, and mutation-verified to catch the exact ordering bug (RR-1LMJSQ) it exists for.'
status: wont-fix
---
