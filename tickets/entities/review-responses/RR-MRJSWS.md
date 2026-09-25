---
id: RR-MRJSWS
type: review-response
title: Retried delete of an absent file still writes the entity
finding: deleteFile always stamped via PatchEntity, producing an audit record and on-update automations per retry.
severity: minor
resolution: deleteFile skips the stamp when the file was absent and the stored value already equals the computed stamp (sameStamp); a stale value is still repaired. TestService_DeleteAbsentFileDoesNotWrite.
status: addressed
---
