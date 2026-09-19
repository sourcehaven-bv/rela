---
id: RR-2UZZAO
type: review-response
title: 'Round-trip tests are vacuous: assert value, not verdict'
finding: All four tests in the `MilkdownEditor comment round-trip` describe block asserted `guarded(w).value`, which on an unedited editor is always the input string. `guardWriteBack` (writeBackGuard.ts:62-72) returns `original` for `unchanged`, `churn-suppressed` AND `drift-blocked` alike, so the assertion compares the input to itself and cannot fail. A serializer that corrupts the author's whitespace passed all 20 tests.
severity: critical
resolution: 'Verified by mutation: replaced the toMarkdown runner with one writing `''<!-- '' + body.trim() + '' -->''` (destroying stored whitespace) and confirmed all 20 tests still passed. Added `expect(g.verdict).toBe(''unchanged'')` to each of the four round-trip tests; re-ran against the same mutant and 2 tests now fail at exactly the whitespace-destroying cases. Restored the correct serializer and confirmed green. The `guarded()` helper doc now states why verdict is the load-bearing half.'
status: addressed
---

The reviewer supplied a concrete mutant and I reproduced it rather than taking
the claim on trust. The failure mode is subtle and worth recording: the guard's
whole purpose is to return the ORIGINAL bytes when it refuses a write, so the
field a test naturally reaches for is precisely the one that cannot distinguish
success from a suppressed corruption.

`rawHtmlPassthrough.test.ts:105` has the same weakness, inherited when this file
was modelled on it. Not fixed here (out of scope, and that file has the corpus
test behind it) but worth a follow-up.
