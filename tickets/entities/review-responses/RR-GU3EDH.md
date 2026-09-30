---
id: RR-GU3EDH
type: review-response
title: A searcher that cannot admit fails at query time, not at wiring
finding: NewVisible accepts a Searcher that ignores Admit or a Service over a non-admitting backend; the error or empty result appears per query.
severity: minor
reason: Both outcomes fail closed (ErrScope or no hits). Rejecting at wiring would forbid decorators, which TestSearchVisibleFields_FailsClosedWithoutProvenance deliberately exercises. Production wires a Service over bleve or linear, both admitting, or pg's native searcher.
status: wont-fix
---
