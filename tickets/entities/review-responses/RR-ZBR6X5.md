---
id: RR-ZBR6X5
type: review-response
title: Guide overstates index use for descending sorts
finding: A descending OrderBy still sorts its last key because the id tiebreak stays ascending.
severity: nit
resolution: The guide now says ascending pages read one index range and a descending sort still sorts its last key.
status: addressed
---
