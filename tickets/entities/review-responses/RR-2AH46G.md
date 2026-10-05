---
id: RR-2AH46G
type: review-response
title: UpdateEntity can revert a concurrent attachment stamp
finding: A whole-entity save reads the row and then writes it unconditionally. A stamp landing between the two is overwritten with the old file value, which can name bytes the attachment service already deleted.
severity: significant
resolution: For types with file properties UpdateEntity writes conditionally on the read version and retries against the fresh row (updateKeepingFileValues). Pinned by TestUpdateEntity_DoesNotRevertAConcurrentStamp (mutation-checked).
status: addressed
---
