---
id: RR-6DWU
type: review-response
title: AC ordering in test comments is 1, 3, 2 — confusing
finding: The comment block reads 'AC1, AC3, AC2'. Either renumber to match flow or reorder the assertion blocks.
severity: nit
resolution: 'Test simplified to a single concern (AC2: bundled FA stylesheet applied). AC1/AC3/AC4 moved to the fixture-level guard. No more numbered ACs in test comments — there''s only one assertion now.'
status: addressed
---
