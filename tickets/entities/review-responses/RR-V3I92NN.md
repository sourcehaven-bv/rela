---
id: RR-V3I92NN
type: review-response
title: 'Design: pending rows need their own listing'
finding: 'RangeFingerprint reports Pending but the only way to find those rows was listing the whole range, so a type with visible: grants would never benefit from the stored sums.'
severity: significant
resolution: 'Plan changed: ListRange takes a RangeList{Limit, PendingOnly}; PendingOnly pushes NOT ok into the query so step 4/5 fetch only the rows they must complete.'
status: addressed
---
