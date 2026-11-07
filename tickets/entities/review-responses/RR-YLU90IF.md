---
id: RR-YLU90IF
type: review-response
title: Floor is still a statistical bound
finding: exhausted >= numRuns/10 still bounds a random total; the fix may only make the flake rarer.
severity: significant
resolution: 'Measured over seeds 1 to 200 at 300 runs: minimum 46, mean 64.5 against 30 (old code: minimum 23, 7 seeds under). A floor failure names its seed and run count for replay.'
status: addressed
---
