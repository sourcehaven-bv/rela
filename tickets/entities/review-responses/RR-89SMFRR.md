---
id: RR-89SMFRR
type: review-response
title: Invalid YAML must keep the strict parser error
finding: If MigrateBytes fails to unmarshal it could replace the humanized parse error with a raw yaml.v3 error.
severity: minor
resolution: MigrateBytes returns the input unchanged when it does not parse, so the existing parser reports the error as today.
status: addressed
---
