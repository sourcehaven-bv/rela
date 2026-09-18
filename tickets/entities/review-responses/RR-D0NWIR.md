---
id: RR-D0NWIR
type: review-response
title: Mount added a third serial await for a load with no dependency on the second
finding: onMounted became three sequential awaits. resolveOutOfPageLinks needs only loadCandidates (to know what is missing) and has no dependency on loadIncomingValue, so sequencing it behind that one adds first-paint latency on forms mounting several pickers — the forms already slowest.
severity: significant
resolution: The two independent loads now run under Promise.all after loadCandidates, with a comment stating the dependency that justifies the ordering.
status: addressed
---
