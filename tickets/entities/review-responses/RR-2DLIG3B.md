---
id: RR-2DLIG3B
type: review-response
title: Two columns mean archived and they can disagree
finding: Apply and Archive read archived from archived_at alone. An old-release resume leaves archived_at on an active tenant. Operator Suspend of an archived tenant rewrote the cause to operator.
severity: significant
resolution: 'Record.Archived requires status suspended and archived_at; Apply, Archive and the API body use it. Operator Suspend keeps cause archive on an archived tenant (tested). No CHECK constraint: an old release can still write the combination, and Archived ignores it.'
status: addressed
---
