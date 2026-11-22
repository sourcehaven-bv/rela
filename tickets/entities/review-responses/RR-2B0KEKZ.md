---
id: RR-2B0KEKZ
type: review-response
title: Denied reversal does not name the transaction
finding: A refused reversal's denied-write line does not say which transaction was being reversed.
severity: nit
resolution: The denied line adds '(reversal of transaction G)'; TestReverse_AuditsDenial pins it.
status: addressed
---
