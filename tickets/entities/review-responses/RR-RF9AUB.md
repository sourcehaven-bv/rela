---
id: RR-RF9AUB
type: review-response
title: Read state once
finding: scopedSortedEntitiesScoped reads a.Meta()/a.Cfg() separately from resolvePageScope's state.
severity: minor
resolution: 'Plan updated: Orderability resolved on the state snapshot resolvePageScope already loads and passed down.'
status: addressed
---
