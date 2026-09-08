---
id: RR-HVUBS7
type: review-response
title: 'PR-B: unparseable cursors compared as garbage instead of restarting'
finding: entityWhere fed an unparseable cursor string into the keyset comparison as the id, silently SKIPPING every row sorting below it — indistinguishable from end-of-results to a paging caller — while the comment claimed 'degrade to a full restart'. splitRelationKey had the same swallow.
severity: significant
resolution: 'Both now genuinely restart: an unparseable cursor omits the keyset condition entirely (entityWhere inline; splitRelationKey returns ok=false and the caller skips the condition), with comments describing the actual behavior and why comparing garbage is worse.'
status: addressed
---
