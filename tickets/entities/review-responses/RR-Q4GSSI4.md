---
id: RR-Q4GSSI4
type: review-response
title: The recheck repeats the whole field check and audits inside the Tx
finding: 'RecheckAtWrite made the manager call CheckFieldWrite again inside the Tx: the full verdict with ACL grant reads, and the denied-write audit record written while the lock was held.'
severity: significant
resolution: FieldWriteGate.WriteRecheck replaces RecheckAtWrite. WriteGate returns a check of the flow edit ceiling only; the permissions its rules ask for are answered before the Tx (Ceiling.Permissions). The manager records the refusal after the Tx returns. Pinned by TestWriteGate_WriteRecheck and TestPatchEntity_RecheckAtWrite.
status: addressed
---
