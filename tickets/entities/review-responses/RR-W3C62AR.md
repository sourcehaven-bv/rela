---
id: RR-W3C62AR
type: review-response
title: Cases in one file shared a Lua state
finding: Globals and module tables set in one case leaked into the next, so case order changed results.
severity: significant
resolution: Each case re-runs the file on a fresh sandbox state and checks the case name matches. The AC7 test pins a LEAK global and a lib/ module count.
status: addressed
---
