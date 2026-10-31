---
id: RR-XNLLQO8
type: review-response
title: The default-after test could fail around midnight
finding: The test used the UTC date while rrule_next without after uses the local date.
severity: minor
resolution: TestRruleNextOnTodaysOccurrence now derives today from the local date, like rrule_next without an after date.
reason: The test now uses the local date like the code.
status: addressed
---
