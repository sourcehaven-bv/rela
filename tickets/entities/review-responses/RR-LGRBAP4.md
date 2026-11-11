---
id: RR-LGRBAP4
type: review-response
title: Two completions at one generation both write
finding: Checking the generation, writing, then moving lets two holders who saw the same generation both write the subject; only one move wins and the loser's values overwrite the winner's.
severity: critical
resolution: 'The engine claims the step first: a compare-and-set stores a claim (generation, caller, time) in the run state before the write, so a second completion gets a conflict before it writes. Change clears the claim; a claim older than two minutes is void. Concurrent test added to the plan.'
status: addressed
---
