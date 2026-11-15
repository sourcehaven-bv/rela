---
id: RR-77ZJCOX
type: review-response
title: Missing tests for related inputs, truncation and multi-type candidates
finding: llmRelated, candidate truncation, multiple to types and parse error mapping had no tests.
severity: significant
resolution: Added handler tests in actions_llm_test.go covering direction, limit, hidden peers and fields, truncation, multi-type candidates, the no-call path and reply errors.
status: addressed
---
