---
id: RR-KLEWD5
type: review-response
title: Field denials not audited
finding: Row denials were audited, manager field denials were not.
severity: significant
resolution: checkFieldWrite records a denied-write with FieldWriteError.AuditSummary (dataentry's format, with attribution); TestFieldGate_DenialIsAudited.
status: addressed
---
