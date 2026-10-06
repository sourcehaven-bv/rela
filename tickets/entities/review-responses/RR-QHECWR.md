---
id: RR-QHECWR
type: review-response
title: Error and response text drops the tail
finding: relation-history errors and the dataentry timeline response print from without the face.
severity: nit
reason: CLI lifetime errors now print ID@face. The dataentry response shape is a wire format; adding the face there is an API change outside this PR, and the caller already addressed the tail.
status: wont-fix
---
