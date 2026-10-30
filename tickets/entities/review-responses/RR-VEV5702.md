---
id: RR-VEV5702
type: review-response
title: Startup warning repeats on every reload
finding: buildApp runs per generation, so the warning is logged again on every config reload.
severity: nit
resolution: 'Intended and documented in the plan: acl.yaml can appear on reload, so each new generation must re-check.'
status: addressed
---
