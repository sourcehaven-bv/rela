---
id: RR-XBZX5
type: review-response
title: Escaped-view failure mode is silent on fs/mem — undocumented
finding: The store.Transactor godoc documents the deadlock/bypass failure modes of writing on the OUTER store inside fn, but not the failure mode of using an ESCAPED view after Tx returns — which on fs/mem silently bypasses txMu serialization (no deadlock, no error, just a lost-update race), the one quiet failure in an otherwise fail-loud design.
severity: minor
resolution: 'Added a sentence to the store.Transactor godoc (internal/store/store.go) naming the escaped-view path as the one misuse that fails silently: fs/mem write without the serialization lock, pg errors against a closed transaction.'
status: addressed
---
