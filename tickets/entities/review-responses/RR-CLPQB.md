---
id: RR-CLPQB
type: review-response
title: Test does not assert day-of-month, missing the off-by-one
finding: Asserting only /Jan/ and /2024/ leaves the day component untested, so the off-by-one bug from RR-TUOOT cannot be caught here. Combine with the locale fix in RR-H3K1D and assert toBe('Jan 15, 2024') exactly.
severity: significant
resolution: Tests now assert /15/ and /2024/ for formatValue/formatCellValue (day-of-month would change under TZ shift) plus a dedicated formatDate test that asserts the exact strings 'Jan 15, 2024' (en-US) and '15 Jan 2024' (en-GB) and a separate test that the day stays preserved across all timezones.
status: addressed
---
