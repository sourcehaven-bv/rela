---
id: RR-6HKC1UA
type: review-response
title: Skip-to-end may drop a complete first line
finding: After skipping past more than the hard cap, skip = true drops the first complete line even when the jump landed on a line boundary.
severity: nit
reason: The jump already reports a gap, and the resync that follows rechecks every channel, which covers that line. Reading an extra byte to tell the cases apart adds code for no change in behaviour.
status: wont-fix
---
