---
id: RR-0QK0BQE
type: review-response
title: Gap analysis keys prefixes by declared case
finding: dataentry/analyze.go and analysis/analysis.go build prefix maps from the declared prefix and look them up by the id's case, so nt- types lose their badge and a manual DOC-12 under doc- reads as a gap.
severity: minor
reason: Pre-existing case-sensitivity in analysis output only, not on the write path. Filed as TKT-MXZ6FBN.
status: deferred
---
