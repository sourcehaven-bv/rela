---
id: RR-KWOOXOR
type: review-response
title: A concurrent run write drops a write's move
finding: A lost compare-and-set on the run in changeRun returned no move and no error, so the edit's event was silently lost.
severity: critical
resolution: changeRunRetried re-reads the run and retries up to three times; tokens already acted on are skipped; a run still lost fails the write with errEventLost.
status: addressed
---
