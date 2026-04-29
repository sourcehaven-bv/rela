---
id: RR-3K96E
type: review-response
title: Missing tests for empty/invalid input on formatDate
finding: No unit tests pinning formatDate('') or formatDate('2024-13-45') behavior. Both call sites guard with typeof===string today, but a contract test would prevent silent regressions.
severity: minor
resolution: Added formatDate test that asserts formatDate('') === null, formatDate('not-a-date') === null, and formatDate('2024-13-45') === null (overflow rejected). Pin contract.
status: addressed
---
