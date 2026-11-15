---
id: RR-FNDDY95
type: review-response
title: Tracer debug args include the exec mode
finding: The exec mode is passed through Args, so the debug log shifted every bind parameter by one.
severity: nit
resolution: TraceQueryStart strips a leading pgx.QueryExecMode from the logged args.
status: addressed
---
