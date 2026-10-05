---
id: RR-NM7E86
type: review-response
title: Default-world bare-id read is a zero-face read the archguard cannot see
finding: 'InWorld reads entity.Ref{ID: id} in the default world, which is GetEntityState(id, "") but not a literal the zero-face guard matches.'
severity: minor
resolution: Added a call-site comment naming TKT-7IZHP0 as the ticket that removes this read. The read is gated by Contains("") first.
status: addressed
---
