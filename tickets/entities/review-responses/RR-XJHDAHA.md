---
id: RR-XJHDAHA
type: review-response
title: Push report nits
finding: refresh counts its merge into the pull counts; the fetch budget is shared with the pull; Prune named the conflict list as the ledger in its error.
severity: nit
resolution: The Prune error now names the remote's state. The shared budget is by design (max_fetch bounds all requests to the remote, documented); refresh merges are real pull writes, so counting them as pulled is correct.
status: addressed
---
