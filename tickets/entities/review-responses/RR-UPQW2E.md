---
id: RR-UPQW2E
type: review-response
title: Orphan keychain item when index write fails
finding: set stored the item before writing the index, with no rollback.
severity: minor
resolution: set deletes a new item when the index write fails. Tested.
status: addressed
---
