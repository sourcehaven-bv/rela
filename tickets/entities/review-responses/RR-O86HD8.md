---
id: RR-O86HD8
type: review-response
title: One realtime typo disables audit check A7
finding: a load error skips the dead-permission check entirely
severity: minor
resolution: Audit scans permissions from raw YAML; A7 is skipped only on unparseable YAML.
status: addressed
---
