---
id: RR-60NRN4K
type: review-response
title: Fail-closed test covers few entry points
finding: The rollback test covered create and patch only.
severity: minor
resolution: Added the delete case; cascade, relation and soft-delete paths share auditTx and emitWrite.
status: addressed
---

The rollback test covered create and patch only.
