---
id: RR-06UH3HU
type: review-response
title: RunList loads every log
finding: With the log in a header property, the run list read and decoded every run's log although it shows none.
severity: significant
resolution: The log is now the header body, which header list reads do not carry; the test asserts the run list has no timeline.
status: addressed
---
