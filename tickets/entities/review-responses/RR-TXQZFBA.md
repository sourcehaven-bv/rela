---
id: RR-TXQZFBA
type: review-response
title: Soft validation warnings dropped
finding: writeRelationTriggerSet discards the soft warnings PatchEntity returns.
severity: minor
reason: A relation write has no response field for another entity's warnings; hard errors still refuse.
status: wont-fix
---
