---
id: RR-AEOK
type: review-response
title: compareValues silently degrades to lexicographic on type mismatch
finding: 'The fallback chain (date → numeric → string) produces plausible-looking lies when one side parses as a date/number and the other doesn''t. Example: due_date=''2026-04-07'' lt ''tomorrow'' falls through to lexicographic and returns true. Users have no signal that the filter is broken.'
severity: critical
resolution: compareValues now uses strict same-type comparison. Returns (false, error) on type mismatch instead of falling through to lexicographic. Caller logs the error and excludes the entity. Tests verify date-vs-non-date and number-vs-non-string cases all return errors.
status: addressed
---
