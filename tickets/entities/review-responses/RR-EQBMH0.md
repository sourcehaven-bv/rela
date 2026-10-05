---
id: RR-EQBMH0
type: review-response
title: DeleteEntityFace computes dropped names without the lock
finding: The names to delete were read before the attachment lock was held, so a concurrent upload could be deleted.
severity: minor
resolution: 'DeleteEntityFace runs releaseUnreferencedFiles: a sweep under the entity lock that deletes only bytes no remaining face references.'
status: addressed
---
