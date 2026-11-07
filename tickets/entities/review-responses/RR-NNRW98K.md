---
id: RR-NNRW98K
type: review-response
title: 'Security: probe must not mutate the shared query template'
finding: ACL query templates may be shared; mutating them would corrupt other reads.
severity: minor
resolution: 'Plan changed: probes copy by value and clone slices before setting EntityType and Faces.'
status: addressed
---
