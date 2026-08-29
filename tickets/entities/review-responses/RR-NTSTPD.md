---
id: RR-NTSTPD
type: review-response
title: Empty in/ne value is not the complement of a populated one
finding: 'filter[tags][in]= and filter[tags][ne]= do not partition the row set: strings.Split("", ",") yields [""], a one-element set containing the empty string rather than an empty set, and the top-of-loop !ok branch special-cases eq+empty only, dropping the row for every other operator. Reviewer recommended treating an empty in/ne as no constraint, matching the relation pass.'
severity: significant
reason: 'Real and correctly diagnosed, but pre-existing and out of scope. Verified against the pre-fix code: the empty-value handling is byte-identical before and after this change — it is the top-of-loop missing-property branch (untouched here) that breaks the complement, not the element-wise comparison. The recommended fix changes behavior for SCALAR properties on a path this bug does not otherwise touch, which is exactly the blast-radius argument used to split TKT-UTJ24Z out. Folding it in would mean a regression there is indistinguishable from this fix. Recorded on TKT-UTJ24Z, which already owns the operator-semantics reconciliation.'
status: deferred
---
