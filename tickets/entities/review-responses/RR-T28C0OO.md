---
id: RR-T28C0OO
type: review-response
title: 'Design: logging tests and 401 sampling'
finding: AC5 only captured slog; a scanner could flood the log with 401s.
severity: minor
resolution: Tests assert the key is absent from the log, the 401 body and the audit row. 401 logging reuses the JWT gate logSampler.
status: addressed
---
