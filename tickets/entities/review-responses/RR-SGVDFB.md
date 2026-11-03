---
id: RR-SGVDFB
type: review-response
title: AC4 hidden-field condition case cannot be built with a valid config
finding: 'Lifecycle conditions go through compileRule/allowRule, which already refuses at load any property under a visible: grant. A test of "a condition on a field the caller cannot see" therefore needs an invalid config. Restate AC4 as the degraded-mode case (new acl.yaml restricts a field the old config reads) and keep redaction as defence in depth.'
severity: minor
resolution: AC4 is restated as the degraded case (a new acl.yaml restricts a field the old config reads). Redaction stays as defence in depth.
status: addressed
---
