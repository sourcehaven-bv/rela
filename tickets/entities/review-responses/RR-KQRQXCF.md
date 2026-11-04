---
id: RR-KQRQXCF
type: review-response
title: Batch test gaps
finding: 'No hidden-source HTTP test, no visible: redaction test, no cardinality/owning/faced tests, no follow_up_error test, no storetest TxRollsBack conformance, no pg batch test.'
severity: minor
reason: Added the hidden-source test. Engine conformance runs on postgres and sqlite (TestCreateBatch_* incl. rollback-only cases); redaction goes through forWireRelated, the single-create path. Cardinality, owning, faced and follow_up_error cases are left for follow-up test work.
status: deferred
---
