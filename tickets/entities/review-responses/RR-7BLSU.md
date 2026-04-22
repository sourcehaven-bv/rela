---
id: RR-7BLSU
type: review-response
title: buildIfMissing busy-waits on a spin loop
finding: '`while (Date.now() < end) {}` pegs CPU. Use a real sleep primitive (Atomics.wait, execSync(''sleep'')).'
severity: minor
reason: buildIfMissing busy-wait runs only on a cold repo with >1 worker. Single build takes seconds; the busy-wait tops out at ~250ms intervals. Minor CPU cost; defer until a flake actually surfaces.
status: deferred
---
