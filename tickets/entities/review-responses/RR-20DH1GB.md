---
id: RR-20DH1GB
type: review-response
title: Two writes and schema drift
finding: Exactly one _carries cannot be checked on create; a subject type that later gains faces drops out of the targets.
severity: minor
resolution: 'Plan changed: no cardinality bound on _carries; drift documented.'
status: addressed
---
