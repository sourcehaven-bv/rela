---
id: RR-2Y3ALS
type: review-response
title: formatDatetime throws on invalid tz where siblings degrade to null
finding: 'formatDatetime calls toLocaleString(locale, {timeZone: tz}) which throws RangeError on a bad zone, unlike sibling formatDate which returns null. Unreachable today (tz always from validated effectiveTimezone or browserTimeZone), but it''s an exported util a future caller could pass an unvalidated zone to. Fix: wrap the toLocaleString call in try/catch and return null on failure.'
severity: minor
resolution: Fixed. formatDatetime's toLocaleString call is now wrapped in try/catch returning null on RangeError (invalid tz), degrading gracefully like formatDate. Test in format.test.ts asserts null (no throw) for tz 'Not/AZone'.
status: addressed
---
