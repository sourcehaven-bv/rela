---
id: RR-45G02BY
type: review-response
title: 'Design: constrain entity type through the address window'
finding: Filtering e.type inside an OR forces a heap visit for every row in the range including out-of-scope types and defeats the covering index.
severity: significant
resolution: 'Plan changed: one UNION ALL arm per scope type over [max(lo, t/), min(hi, t0)); an AllowAll arm with no faces and no hidden fields reads only index columns.'
status: addressed
---
