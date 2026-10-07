---
id: RR-4A7BDS
type: review-response
title: Partial soft delete or restore on fs/mem is not audited
finding: With no rollback, a failure on child 2 left the owner and child 1 changed with no audit record.
severity: significant
resolution: SoftDeleteEntity/RestoreEntity track a familyChange and audit what really changed when the Tx fails. TestSoftDelete_PartialFailureIsAudited.
status: addressed
---
