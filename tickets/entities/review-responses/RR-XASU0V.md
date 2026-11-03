---
id: RR-XASU0V
type: review-response
title: An already-unreachable entry should not get a second button problem
finding: unreachable() already refuses an entry at key[j] (internal/realtimecfg/branches.go:176-178). The button reachability check would add a second problem at key[j].button for the same cause. Skip the button check when the entry itself was refused, as AC 4 already does for a broken guard.
severity: minor
resolution: The button checks run after unreachable() and skip an entry it already refused, and TestButtons_Guards gains a reports-once case.
status: addressed
---
