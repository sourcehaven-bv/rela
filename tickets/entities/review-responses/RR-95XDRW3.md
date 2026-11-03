---
id: RR-95XDRW3
type: review-response
title: Fake ignores query parameters on view and get
finding: ?world= and ?include= were answered as if absent, though the header promises 501 for anything not modelled.
severity: minor
resolution: Every route except a list and the inert routes answers 501 for a query parameter.
status: addressed
---
