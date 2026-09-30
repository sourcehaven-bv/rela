---
id: RR-CK851X
type: review-response
title: Restore guard denial had an empty from state
finding: The GuardError built by EnforceRestore had no From, so the message and audit reason read ""->"established".
severity: minor
resolution: The GuardError now names the refusing edge (From, To, Permission).
status: addressed
---
