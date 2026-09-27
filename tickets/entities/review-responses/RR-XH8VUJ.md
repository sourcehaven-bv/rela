---
id: RR-XH8VUJ
type: review-response
title: DerivedDropped outcome names only the index hash
finding: The operator cannot tell which type a dropped index served.
severity: nit
reason: Once the spec is gone the name is all that remains, as on pgstore; the stored DDL could be parsed but that is not worth it for a log line.
status: wont-fix
---
