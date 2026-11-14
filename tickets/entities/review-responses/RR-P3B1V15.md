---
id: RR-P3B1V15
type: review-response
title: Scalar ~= disagrees with Go on off-type stored values
finding: 'cranky: PropNotEqualOrEmpty used loose equality, so a list or number stored under a string declaration was dropped by the store but kept by Go (which binds it to nil, and nil ~= v is true).'
severity: significant
resolution: 'Addressed: Scalar now applies to PropNotEqualOrEmpty on graphquerynaive, pgstore and sqlitestore (NOT of the scalar string equality), LowerCondition emits it, a storetest case and the graph differential cover it, and the list fixture has list, number and unset statuses.'
status: addressed
---
