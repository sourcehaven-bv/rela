---
id: RR-ZUVW8WM
type: review-response
title: Rows mirrored from another remote are pushed to this one
finding: 'A local type mapped by two remotes with different refs: rows pulled from remote A have no ref for remote B, so a push to B creates them there.'
severity: minor
resolution: Documented in remote-sync.md with share as the way to leave them out. A guard on other external refs would also skip legitimate rows that link to other systems.
status: addressed
---
