---
id: RR-P49CP98
type: review-response
title: afterCommitCtx is a deny-list with typed-nil sentinels
finding: afterCommitCtx cleared three keys by name with typed nils; emitWrite dereferenced a typed-nil txBatch and a new transaction key would leak.
severity: significant
resolution: 'The package keeps its transaction state (commit list, txBatch, heldAudit) under one txScope key read through scopeOf, which never returns nil. afterCommitCtx sets the empty scope and calls store.ContextOutsideTx: one value per package, no typed nils.'
status: addressed
---
