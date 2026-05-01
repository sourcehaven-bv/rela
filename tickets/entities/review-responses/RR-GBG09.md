---
id: RR-GBG09
type: review-response
title: Gratuitous assignee in TASK-002 fixture
finding: 'e2e/tests/fixtures.ts TASK-002 fixture sets assignee: Bob — irrelevant noise for the test. Drop it or align other task fixtures.'
severity: nit
resolution: Removed assignee from TASK-002 fixture. Test cares about title and the implements-relation absence only.
status: addressed
---
