---
id: RR-9Y2DYS
type: review-response
title: Picker failure reported as nothing fixable
finding: Any non-zero exit of govulncheck-fixable.sh (e.g. jq parse error) logged 'No fixable vulnerabilities'.
severity: minor
resolution: The workflow captures rc; 1 means nothing fixable and any other code logs 'Fix picker failed (rc=N)'. A test pins that truncated JSON exits with a code other than 1.
status: addressed
---
