---
id: RR-6KWVWY8
type: review-response
title: Subject runs have no cap
finding: SubjectRuns reads every _carries edge and header of a subject and builds the detailed view for each, with no limit or truncation flag.
severity: minor
reason: Bounded in practice by how often one subject is run. Recorded on TKT-FWQZGAM with the other M1d list-cost items; it needs a paginated response shape.
status: deferred
---
