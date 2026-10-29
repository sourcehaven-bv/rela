---
id: RR-G5QVPL
type: review-response
title: Preset boolean spelling differs from the rest of realtime.yaml
finding: The key checker accepts yes, on and the rest for config booleans because yaml.v3 decides (internal/realtimecfg/keys.go:166-174). A preset that requires the !!bool tag refuses yes. Matching the post's rule is defensible, but say so in the docs, or the operator gets surprised.
severity: nit
resolution: The plan keeps the post's rule for preset booleans and the docs key table now says a preset boolean must be true or false.
status: addressed
---
