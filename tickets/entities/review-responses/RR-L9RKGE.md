---
id: RR-L9RKGE
type: review-response
title: Print a stderr notice when --run-as or RELA_RUN_AS is active
finding: A RELA_RUN_AS left in a shell profile silently changes the identity of every interactive write command. A one-line "acting as X (asserted by Y)" on stderr for writes makes that visible.
severity: nit
resolution: Write commands print "acting as X (asserted by Y)" on stderr when run-as is active.
status: addressed
---
