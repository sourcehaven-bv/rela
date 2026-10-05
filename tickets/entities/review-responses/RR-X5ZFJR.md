---
id: RR-X5ZFJR
type: review-response
title: writePrepRow used for a liveness read
finding: Some callers use entityReader.writePrepRow only to check that a row exists and not to prepare a write.
severity: nit
reason: 'The name states the contract that matters: an ungated raw read that must never decide what is served. A liveness check needs exactly that read. A second name for the same raw read would make the allowlisted raw surface harder to audit.'
status: wont-fix
---
