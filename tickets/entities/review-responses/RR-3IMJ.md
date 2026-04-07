---
id: RR-3IMJ
type: review-response
title: $today as literal value cannot be filtered
finding: Users with literal $today values (e.g. tags or status enum) cannot filter for them — there's no escape syntax. Document that $-prefixed values are reserved or add escape support.
severity: significant
reason: No escape syntax for literal $-prefixed values. Documented as a known limitation in the user guide. If this becomes a real problem, we can add backslash escaping or a {{$today}} alternative syntax.
status: deferred
---
