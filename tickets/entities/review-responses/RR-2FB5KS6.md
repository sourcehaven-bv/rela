---
id: RR-2FB5KS6
type: review-response
title: Capability set after construction from a different source
finding: SetAtomicBatch mutates App after construction and the wiring test is weak; compute the flag from the store the manager writes to.
severity: minor
resolution: No setter. The capability is store.TxRollsBack on the store the App and the manager share, read in newBatchHandler and handleV1Schema. TestBatchSQLite_CapabilityIsAdvertised and TestBatch_UnsupportedStoreIs501.
status: addressed
---
