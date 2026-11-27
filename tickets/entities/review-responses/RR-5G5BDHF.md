---
id: RR-5G5BDHF
type: review-response
title: 'Design: lineage ids of clears from a pre-pass'
finding: 'A clear found only in the Tx has no lineage id and would get a wrong or missing history record.'
severity: significant
resolution: 'Lineage ids are read before the Tx from a pre-pass of the same check. A clear in the Tx that the pre-pass did not see fails the write with a conflict, so history is never filed under the wrong lineage.'
status: addressed
---
