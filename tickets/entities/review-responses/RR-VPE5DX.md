---
id: RR-VPE5DX
type: review-response
title: Integer and boolean where untested; ParseInt accepts +05
finding: No conformance case for non-scalar integer/boolean equality; +05 would match nothing.
severity: minor
resolution: wherePredicate canonicalises with FormatInt; storetest where_on_integer_and_boolean passes on all backends.
status: addressed
---
