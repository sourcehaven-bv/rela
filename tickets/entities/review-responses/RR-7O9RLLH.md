---
id: RR-7O9RLLH
type: review-response
title: Test files could not require helpers from tests/
finding: require searched only lib/, so shared test helpers had to live with production modules.
severity: minor
resolution: NewSandbox takes module directories; rela test searches lib/ then tests/. TestRequireFromTests covers it.
status: addressed
---
