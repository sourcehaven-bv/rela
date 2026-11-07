---
id: RR-0JGLHMG
type: review-response
title: A failed fetch on a deactivation event leaves roles in place
finding: '[security] Every error keeps the stored roles and relies on Pratique retries, which dead-letter after about 12 attempts. A revocation can then be lost.'
severity: minor
resolution: For membership.deleted a failed fetch now clears the roles, since the event proves the membership is gone (TestIdpSyncAction_DeletedClearsWithoutAPI). For membership.updated an error still keeps roles, because that event also covers role changes; the docs name Pratique's dead-letter log as the signal.
status: addressed
---
