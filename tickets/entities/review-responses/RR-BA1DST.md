---
id: RR-BA1DST
type: review-response
title: File-name checks depend on the host MIME table
finding: With an unknown output type any extension was accepted; dotted names such as audit-v1.2 were refused; results vary by host.
severity: minor
resolution: An unknown output type now gets .bin and refuses other extensions. A suffix that names no known type counts as part of the name. The host dependency is documented. A transform-level extension key is left for later.
status: addressed
---
