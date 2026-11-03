---
id: RR-9JZXTS0
type: review-response
title: Migration test files missing and pg version wrong
finding: sqlite shape tests per rung, channelcause_test version asserts and pg status_test target 39 must change; the plan said pg schema version 39.
severity: minor
resolution: All files listed; sqlite schemaVersion 33, pg migration target 40; new channelprevious tests for both.
status: addressed
---
