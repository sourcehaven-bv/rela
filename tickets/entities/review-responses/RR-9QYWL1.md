---
id: RR-9QYWL1
type: review-response
title: Runtime syslog write errors are silent
finding: slog discards handler errors, so a socket that disappears after startup empties the log without notice; the guide overstated the guarantee.
severity: minor
resolution: 'Guide narrowed: startup fails loudly, later write failures are dropped until restart.'
status: addressed
---
