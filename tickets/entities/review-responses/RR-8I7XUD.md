---
id: RR-8I7XUD
type: review-response
title: intraIns:1 makes uFuzzy matching exponential, freezing the editor tab for up to 55 seconds
finding: '`new uFuzzy({ intraIns: 1 })` was the one non-default option, and it made matching exponential in needle length when a repeated-character run in the needle met one in the haystack (the compiled regex backtracks). Measured independently against the installed @leeoniya/ufuzzy 1.0.19: needle 24 chars = 27s, 32 chars = 3.2s, 64 chars = 55s, all 0ms on the defaults. The work is synchronous on the main thread inside rankEntities, and the 150ms debounce bounds how OFTEN matching starts, not how long one run takes. Stored cross-user path: any principal who can write a display property sets it to a long repeated run (a plain long title no validator rejects); any other principal who can read that entity and types a long query after @ stalls their editor for seconds per keystroke, while holding unsaved document state.'
severity: significant
resolution: 'Removed `intraIns: 1`; uFuzzy now runs on its defaults. A length bound was considered and rejected: measured, the blowup begins around 16 characters, which is shorter than the queries this feature exists to serve. Verified the option only bought single-typo tolerance (`rankng` -> `ranking`); the motivating case `fancyreport` -> `FancyReport` works on the defaults because the split is on the needle''s punctuation, not on camel-case in the haystack. All 43 ranking tests still pass. Pinned by a COST assertion (a worst-case 64-char repeated-run needle must rank in under 500ms) because every correctness fixture passes either way; mutation-verified, reintroducing the option hangs the suite past 90s instead of failing, which the test comment calls out.'
status: addressed
---
