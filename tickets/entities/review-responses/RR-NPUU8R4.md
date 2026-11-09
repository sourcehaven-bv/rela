---
id: RR-NPUU8R4
type: review-response
title: Stuck rows starve the max_fetch budget
finding: Conflicted, failed and deferred rows came back first in address order every run and could use up max_fetch, so later rows were never fetched.
severity: significant
resolution: The ledger holds the source hash a row stopped at; rows held at an unchanged hash are pulled last. The report counts rows waiting on the budget. TestRun_HeldRowsGoLast.
status: addressed
---
