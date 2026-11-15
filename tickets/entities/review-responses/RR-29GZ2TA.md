---
id: RR-29GZ2TA
type: review-response
title: Existing relations and max cardinality
finding: Candidates already related should be marked existing, not proposed; max should respect the relation max_outgoing.
severity: nit
resolution: Candidates already related are marked related and are not proposed again. Config load rejects max above the relation's max_outgoing.
status: addressed
---
