---
id: RR-T420JHP
type: review-response
title: 'Design: document that a branch cannot loop out of its region'
finding: Rework loops that leave a region are refused without a documented alternative.
severity: nit
resolution: 'Plan updated and the guide will say so, with the alternative: a join when: that fires on a rejection, followed by a guarded done branch on the join.'
status: addressed
---
