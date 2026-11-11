---
id: RR-BNM4ZMM
type: review-response
title: Check and write are not atomic
finding: A run that starts between the ceiling check and the write does not stop that write.
severity: minor
resolution: Closed on postgres and sqlite with the store Tx. A field patch whose gate reads flow runs asks the flow edit ceiling again inside the persist Tx (FieldWriteGate.WriteRecheck), reading the runs on the transaction's own handle. The engine's run Create and Update run inside a store Tx on the transaction's handle (txFlowRuns). The write lock (pg advisory lock, sqlite write mutex) orders them. fs keeps no isolation and is documented in GUIDE-flows. Pinned by TestOutsideEdits_RunStartedDuringPatchFailsClosed and TestOutsideEdits_RunWriteTakesWriteLock on both database tags.
status: addressed
---
