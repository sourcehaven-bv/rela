---
id: RR-3ZBYFPD
type: review-response
title: A test decodes an opaque cursor
finding: TestScriptReader_ListRelationsPage decodes NextCursor and compares it to the last served key which bakes in the memstore cursor format.
severity: nit
reason: The decode is the direct pin of the security property (the cursor never encodes a hidden key) on the backend the test runs on. The test also resumes from each cursor and checks that nothing is skipped or repeated; the new hidden-edge test checks resumption without decoding.
status: wont-fix
---
