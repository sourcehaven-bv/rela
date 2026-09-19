---
id: RR-ASNIG5
type: review-response
title: 'Acceptance criteria 8 and 9 have no design behind them'
finding: 'AC-8 (report cardinality bounds not swapped alongside the endpoints) does not say what not swapped means - whether it accounts for the nil-means-0-or-unbounded normalization effectiveBound applies (shapecompare.go:320-330) - and the plan''s Approach suppresses only the two per-side endpoint comparisons, leaving the four cardinality comparisons to fire and reproduce the multi-finding noise the ticket exists to remove. AC-9 (report and leave edges that do not typecheck after reversal) requires resolving each endpoint''s entity type, a per-row lookup of exactly the class CLAUDE.md''s collection-reads rule forbids, and it is absent from the Approach entirely. AC-9 also makes the result a PARTIAL reversal, which compounds the idempotence problem: a re-run then sees a mix it cannot classify.'
severity: significant
status: open
---
