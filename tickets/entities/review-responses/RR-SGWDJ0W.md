---
id: RR-SGWDJ0W
type: review-response
title: Docs overstate when the state is erased
finding: The metamodel guide said the key is always erased when the job ends, but a queue that drops a job past its window never calls the handler.
severity: minor
resolution: The guide now names that case and the 24-hour postgres sweep that removes the row.
status: addressed
---
