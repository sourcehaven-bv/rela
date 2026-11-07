---
id: RR-KJ724BT
type: review-response
title: Final-attempt erase uses a cancelled context
finding: The erase after the final attempt used the job context, which may already be cancelled, so the key stayed.
severity: minor
resolution: eraseDeleted uses context.WithoutCancel with a one-minute timeout.
status: addressed
---
