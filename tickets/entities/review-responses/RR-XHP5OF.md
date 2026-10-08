---
id: RR-XHP5OF
type: review-response
title: 'Design: read list logged twice at server startup'
finding: applyCommandConfinement logs the list and Describe() repeats it in the 'external command confinement' line. Keep one.
severity: nit
resolution: rela-server no longer logs the read list itself; the 'external command confinement' line (Runner.Describe) is the single report. The server only warns about a listed path that does not exist.
status: addressed
---
