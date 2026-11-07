---
id: RR-ZFWRLIS
type: review-response
title: schedules.yaml run_as can name system:flow
finding: 'The scheduler validates run_as with a callback that accepts every system: name, so a scheduled script could run as the engine.'
severity: significant
resolution: 'Plan changed: principal.ValidateRunAs refuses system:flow for every caller, with a test per run_as site.'
status: addressed
---
