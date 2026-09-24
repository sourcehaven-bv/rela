---
id: RR-VPUYU9
type: review-response
title: List equality ignores non-string elements
finding: je.type = 'text' matches only string elements while graphquerynaive compares Stringify(item); pgstore's ? operator behaves the same.
severity: minor
reason: Same divergence in both SQL backends; fixed together with a harness extension in BUG-LCHDSR.
status: deferred
---
