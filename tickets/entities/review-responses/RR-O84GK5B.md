---
id: RR-O84GK5B
type: review-response
title: No ACL test for the unconditioned LLM path
finding: The no-condition LLM candidate read has no test with a hidden row.
severity: minor
reason: The window cut after the row gate is pinned by TestScopedHeaders_WindowReadsOneWindow, and this path reuses the existing lowered read on scopedQuery.
status: wont-fix
---
