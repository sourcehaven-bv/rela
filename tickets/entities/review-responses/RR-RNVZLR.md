---
id: RR-RNVZLR
type: review-response
title: 1xx responses taken as final status
finding: A WriteHeader(103) would be recorded as the status and carry Server-Timing.
severity: nit
resolution: Informational codes except 101 are passed through without recording (test InformationalStatusIsNotFinal).
status: addressed
---
