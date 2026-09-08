---
id: RR-04KFJA
type: review-response
title: PrincipalStore claimed to be consumer-side while embedding a producer interface
finding: The interface embeds store.GraphQueryer and its doc called that consumer-side on purpose.
severity: nit
resolution: 'Doc corrected: it names exactly the graph-query surface store.GraphQueryHeaders takes; the embed stays because that helper needs the full GraphQueryer.'
status: addressed
---
