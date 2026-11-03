---
id: RR-7W7TM5
type: review-response
title: Operator colours with alpha break the AA guarantee
finding: 'The plan accepted #rrggbbaa for operator colours and computed the text colour on the opaque part. A translucent fill blends with whatever is behind it, so the chosen text colour cannot be proven to reach 4.5:1.'
severity: significant
resolution: 'Plan changed: operator colours and dark overrides accept only #rgb and #rrggbb.'
status: addressed
---
