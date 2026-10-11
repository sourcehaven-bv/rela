---
id: RR-XH9DHI
type: review-response
title: Failed batch after thread move left no hint that threads already moved
finding: If the operator abandoned the migration after a failed batch, threads sat at a face no row occupies with no report.
severity: minor
resolution: The applyMoves error now states that the batch's comment threads already moved and that a re-run is needed to move the rows after them.
status: addressed
---
