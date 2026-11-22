---
id: RR-WGGX1VL
type: review-response
title: The read-only check judges a different path than the router uses
finding: readOnlyAllows checked the decoded and cleaned path while ServeMux matches the escaped path. An encoded PUT or POST could pass the read-only check as a subscriptions or webhook request and reach another write route.
severity: significant
resolution: The PUT and POST exceptions now pass only when RawPath is empty and the path is already clean. TestReadOnly adds escaped and unclean paths.
status: addressed
---
