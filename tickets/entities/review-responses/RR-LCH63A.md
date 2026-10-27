---
id: RR-LCH63A
type: review-response
title: 'Design: store read and header read are not one snapshot'
finding: An entity deleted between the tag query and the header batch is reported as neither live nor deleted for that call.
severity: minor
reason: A race of one call against a concurrent delete; the next call reports it deleted. A snapshot across the visibility layer would need a store transaction around a gate, which the Tx rule forbids.
status: wont-fix
---
