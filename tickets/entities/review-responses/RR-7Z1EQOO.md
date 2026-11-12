---
id: RR-7Z1EQOO
type: review-response
title: loadActions skips data-entry validation
finding: The job runner parses data-entry.yaml actions without migration detection or ValidateConfig, unlike the data-entry server.
severity: nit
reason: 'Jobs run only in rela-server and the desktop app, which validate data-entry.yaml at boot. The fields a job relies on are re-checked per attempt: flowActionUsable refuses set/request/available_on/bypass, the permission fails closed and secrets are filtered at runtime. Sharing the validator means moving it out of internal/dataentry, which is out of scope.'
status: wont-fix
---
