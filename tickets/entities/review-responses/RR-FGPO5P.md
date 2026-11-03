---
id: RR-FGPO5P
type: review-response
title: Round-cap subtest relies on the initial rotation cursor and URL sort order
finding: 'live_busy_test.go:589 assumes the run-channel pair is kept and the board pair is cut. That holds only because presenceRotate is 0 and ''/api/v1/tickets/TKT-1/_channels/run'' sorts before ''/api/v1/tickets?busy=run'' (''/'' < ''?''). A setup change that overflows a topic round first would flip it and fail confusingly. Fix: assert which pair was cut (for example isPending on board and not on run) before the frame checks, or set presenceRotate explicitly in the subtest.'
severity: nit
resolution: 'Fixed: the round-cap subtest sets presenceRotate to 0, explains the sort order, and asserts that the board is pending and the run channel is not (or is, for the budget case) before the frame checks.'
status: addressed
---
