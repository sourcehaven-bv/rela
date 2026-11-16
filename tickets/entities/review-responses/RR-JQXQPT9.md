---
id: RR-JQXQPT9
type: review-response
title: No tests pin the batch row gate
finding: Fixtures read every ticket, so an unreadable row, a face mismatch and a when on a hidden property were untested.
severity: minor
resolution: TestLLMBatch_GatedRowsAreNotSent covers an unreadable type, a non-served face and a when on a hidden property; each asserts the rows are skipped and the provider is not called.
status: addressed
---
