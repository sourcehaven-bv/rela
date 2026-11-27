---
id: RR-27VWMO1
type: review-response
title: Timeout message does not say why the match failed
finding: waitForEvent reports only what it waited for; a missing modify and a modify with another Op look alike until the events-seen dump is read.
severity: minor
reason: The failure message already dumps every event seen, which tells the two cases apart.
status: wont-fix
---
