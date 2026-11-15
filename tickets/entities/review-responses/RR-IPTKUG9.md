---
id: RR-IPTKUG9
type: review-response
title: scopedQuery returns two error-typed values
finding: scopedQuery returns errClass and err; a small struct would be clearer.
severity: nit
reason: The shape mirrors TKT-F2EJQY8's split of the same switch, which keeps the expected merge conflict small.
status: wont-fix
---
