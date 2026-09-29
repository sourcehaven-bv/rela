---
id: RR-R42SRB
type: review-response
title: Audit I/O inside the Tx
finding: A denial inside the Tx writes its denied-write record while the Tx is open.
severity: nit
resolution: Matches authorizeCascadeRelations; the audit sink is a local append, not slow external I/O. Documented in the authorizeFamilyDelete godoc.
status: addressed
---
