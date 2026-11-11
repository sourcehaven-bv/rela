---
id: RR-RNIBW94
type: review-response
title: No record of who started or cancelled a run
finding: '[security] The audit records of a manual start or cancel carried only system:flow and the run_as identity, not the principal who asked, so repudiation was possible.'
severity: significant
resolution: audit.FlowStep and Record gained By. FlowControl stamps the caller and the engine carries it on every write of that request. TestStamp pins it.
status: addressed
---
