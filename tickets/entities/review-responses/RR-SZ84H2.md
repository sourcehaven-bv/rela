---
id: RR-SZ84H2
type: review-response
title: cmdexec execute comment no longer describes args[0]
finding: After confine, args[0] is the sandbox wrapper, not the configured command; the comment is the stated reason for the suppression.
severity: nit
resolution: 'Reworded: args are the sandbox wrapper, the configured command, or runner-owned temp paths; request data reaches the command only on stdin or in the in-file.'
status: addressed
---
