---
id: RR-Z12KRD
type: review-response
title: Handler prelude obscured the scope-then-resolve control flow
finding: handleV1NextActionGet declared err up front, conditionally assigned it from the scope binder, then guarded Resolve behind if err == nil with sug/found in a separate var block — four statements to say 'if scoping fails, skip resolving'.
severity: nit
resolution: 'Rewritten as an early return: scope, on error writeNextActionError and return; then Resolve. The error mapping is a free function (writeNextActionError) shared by both sites, since App is at its plimsoll cap.'
status: addressed
---
