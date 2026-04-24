---
id: RR-TTZ7A
type: review-response
title: Negative tests don't verify zero side effects
finding: TestCreateEntity_CustomIDRejectedForSequential only checked REQ-042 wasn't persisted -- a regression that silently substituted a generated REQ-001 would pass. TestCreateEntity_CustomIDRejectedForShort didn't check persistence at all.
severity: minor
resolution: 'Added a countEntities helper and asserted count == 0 in both negative tests. Kept the specific-ID-absence check so failure mode is diagnosable: first assertion localises the bug; second assertion catches silent-substitution regressions.'
status: addressed
---
