---
id: RR-3RRVYB
type: review-response
title: Breadth guard had 84% slack; a 1,200-row over-fetch passed
finding: 'The RR-HKHPYG guard used a flat constant nestedRelationIDLimit = 2 * nestedNodeBudget = 4000, measured against ~2,550 ids of legitimate traffic. That left ~1,450 ids of slack (84% of the real cost). Code review demonstrated by mutation that a bounded over-fetch of 1,200 extra visible children passed the test: the defect''s whole CLASS was only caught above ~1,450 rows. The comment claimed the constant ''separates proportional-to-rendered from proportional-to-visible'', which it did not do arithmetically - it separated them only for over-fetches large enough to clear the fitted threshold.'
severity: critical
resolution: 'Replaced the flat constant with a bound that is a function of the EMITTED tree, counted from the response body: limit = emitted * 5 / 4. The measured legitimate ratio is 1.086 ids per emitted row, so the allowance of 1.25 leaves ~13% margin instead of 84%. Mutation-tested sensitivity is now recorded in the constant''s doc comment: +600 rows caught, +300 missed (was: +1200 missed). The bound scales down with the render instead of staying fixed, so it cannot silently widen when the fixture shrinks.'
status: addressed
---
