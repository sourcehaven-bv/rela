---
id: RR-TUOOT
type: review-response
title: Off-by-one-day for date-only strings west of UTC
finding: 'new Date(''2024-01-15'') parses as 2024-01-15T00:00:00Z, then toLocaleDateString renders in local time. With TZ=America/Los_Angeles the helper returns ''Jan 14, 2024'' for input ''2024-01-15''. The bug existed in the old toLocaleDateString() call but was hidden by the ambiguous numeric output; the new month-name format makes it obvious. Fix: parse YYYY-MM-DD components manually with new Date(y, m-1, d) so construction happens in local time, OR set timeZone: ''UTC'' in DATE_FORMAT_OPTIONS. Add a TZ-pinned test asserting day stays 15.'
severity: critical
resolution: Added parseDate() helper that detects YYYY-MM-DD via regex and constructs new Date(y, mo-1, d) in local time. Other formats (ISO datetime) still fall through to new Date(value). Verified by running test suite under TZ=America/Los_Angeles and TZ=Pacific/Pago_Pago (UTC-11) — all tests pass, day-of-month preserved. Also rejects overflow values (2024-13-45 returns NaN instead of silently rolling into 2025).
status: addressed
---
