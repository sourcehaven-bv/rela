---
id: RR-RQ0M3G6
type: review-response
title: Non-conflicted file answered 500 while a missing one answered 404
finding: '[security] ParseConflictedFile ran before the gate, so a file without markers gave 500 parse_failed and a missing path 404: an existence oracle for hidden entities on detail and resolve.'
severity: significant
resolution: parseConflictOr404 answers every parse failure with the uniform conflict_not_found 404. TestV1Conflicts_HiddenAnswersMatchMissing compares both routes with a missing file.
status: addressed
---
