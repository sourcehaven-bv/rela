---
id: RR-N7NF4V
type: review-response
title: The weekly runner did not verify the sandbox
finding: If bubblewrap breaks on the runner, the command route fails closed and the trace changes shape quietly.
severity: minor
resolution: A bwrap check step, as in ci.yml, fails the run first.
status: addressed
---
