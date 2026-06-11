---
id: RR-IP4ZIK
type: review-response
title: Walk-test oracle conflates mux 404 with handler-emitted http.NotFound
finding: isStdlibNotFound matches the stdlib '404 page not found' body, but registered handlers (handleEntityHelp for unknown types, the /api/v1/ catch-all for unregistered _-paths) emit the identical body. A future negative-path probe would falsely fail with 'route is not registered'.
severity: significant
resolution: Added a loud CONSTRAINT comment on the probe table (no probes that legitimately resolve to handler-emitted http.NotFound) and a matching caveat on isStdlibNotFound explaining the conflation and the table constraint.
status: addressed
---
