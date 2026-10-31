---
id: RR-ZMFB7RP
type: review-response
title: A 500-row buffer breaks the read-ahead bound of a Lua list_entities with a limit
finding: 'Design review: buffering 500 rows before the first yield makes a script that reads five rows pull and redact 500.'
severity: significant
resolution: redactingSeq pages are 64 rows, then headerGateChunk (512). An early stop reads at most one page. TestRedactingSeq_EarlyStopReadsOnePage pins it. The first page is 64, not 16, so the budget fixture's 10 and 50 rows cost the same number of primes.
status: addressed
---
