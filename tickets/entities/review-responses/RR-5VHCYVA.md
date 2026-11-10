---
id: RR-5VHCYVA
type: review-response
title: Done branches are not crash-safe
finding: The mark was cleared after the outputs, before the done branches were followed. A crash between the two left the run waiting in a state it should have left.
severity: critical
resolution: 'The mark now means unsettled: it is cleared only by rest() when no done branch applies, and settle() loops over moves until then.'
status: addressed
---
