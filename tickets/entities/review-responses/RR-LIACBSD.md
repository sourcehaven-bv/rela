---
id: RR-LIACBSD
type: review-response
title: Test hard-codes the hop bound
finding: The chain test restated the bound.
severity: minor
reason: The test now pins the stored depths 0 to 7, which is the observable contract; exporting the constant only for the test would add API.
status: wont-fix
---
