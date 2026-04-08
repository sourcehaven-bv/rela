---
id: RR-2NRF
type: review-response
title: useUrlFilterSync tests miss collision and rapid-write cases
finding: 'The 11 tests cover the happy path well but don''t exercise: (a) external nav whose query happens to stringify to lastWrittenSig (the bug from RR-XO1V), (b) rapid successive writeToQuery calls and which signature wins, (c) non-filter query change (e.g., page increment) producing a stringification difference but not warranting a re-read. Add cases once RR-XO1V is fixed.'
severity: minor
resolution: 'useUrlFilterSync.test.ts adds 3 new test cases: (a) RR-XO1V collision regression (value containing &/= doesn''t false-match an unrelated two-key external nav), (b) rapid successive writeToQuery calls (last one wins, router.replace reflects the final state), (c) non-filter query change doesn''t erroneously corrupt filter state. 14 tests total now.'
status: addressed
---
