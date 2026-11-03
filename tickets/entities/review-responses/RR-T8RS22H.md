---
id: RR-T8RS22H
type: review-response
title: Redundant compact check in OpenChannel
finding: presentationView already narrows session.display to compact or absent.
severity: nit
reason: The comparison is explicit at the one place that turns config into the prop, and the type keeps the library's presentation prop unchanged.
status: wont-fix
---
