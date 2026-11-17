---
id: RR-TB8QH7Z
type: review-response
title: fs-import and db load still write their audit record to JSONL on sqlite
finding: rela db import-fs and db load record fs-import through the JSONL sink.
severity: minor
reason: Moves with the in-transaction append in TKT-TU0VZ4Y.
status: deferred
---
