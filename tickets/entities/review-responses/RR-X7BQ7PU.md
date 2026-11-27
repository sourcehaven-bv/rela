---
id: RR-X7BQ7PU
type: review-response
title: Field gate skipped on trigger write
finding: cascadeWrite bypasses checkFieldWrite, so visible/writable field rules do not judge trigger values.
severity: minor
reason: 'Same as update automations: rules are config, not caller input; documented in the godoc and guide.'
status: wont-fix
---
