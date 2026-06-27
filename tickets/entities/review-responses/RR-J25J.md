---
id: RR-J25J
type: review-response
title: writeVisibleError error-mapping paths untested (504, 500 branches)
finding: 'writeVisibleError''s 504-on-DeadlineExceeded and 500-on-other-error branches have no test coverage. A future refactor that swaps the mapping (e.g. always 500) breaks the contract silently. CI is green; test plan claims AC1-AC7 pinned; error-mapping contract isn''t pinned by any test. Fix: add a fakeGate to acl_get_test.go whose Visible returns a configured error; drive each branch (Canceled → no body, DeadlineExceeded → 504, generic → 500 with acl_query_failed code).'
severity: significant
resolution: Added fakeGate + TestACLGet_WriteVisibleErrorMapping with three sub-cases (Canceled -> empty body, DeadlineExceeded -> 504, generic err -> 500). All three branches now exercised.
status: addressed
---
