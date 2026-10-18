---
id: RR-POQ1KF
type: review-response
title: RunAction checks requireOpen before the CAS so a concurrent close still lets the action run
finding: C3 step 6 judges open from the head loaded in step 1, and SetAction does not check channel state. A close that lands before step 9 lets a promote write to the graph from a closed channel, contrary to D7. Re-check the head after the pending CAS or make SetAction refuse non-open channels.
severity: minor
resolution: SetAction refuses a channel that is not open inside its CAS (ErrNotOpen, 409 channel_not_open).
status: addressed
---
