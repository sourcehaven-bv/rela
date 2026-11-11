---
id: RR-C0TBJSW
type: review-response
title: Recheck inside the write Tx reads on a second pool connection
finding: The check the manager repeats inside the write Tx and the rename guard read the flow runs through the pool. pgstore.Tx holds a pool connection while it waits for the advisory lock, so with enough queued writers the lock holder could wait forever for a second connection.
severity: critical
resolution: 'The manager puts the transaction''s view on ctx (store.ContextWithTxView). txFlowRuns.List reads on the view''s own handle when ctx carries one (pg: the tx; sqlite: the Tx conn), and the ceiling and the rename guard read through txFlowRuns. No pool connection is taken while the write lock is held.'
status: addressed
---
