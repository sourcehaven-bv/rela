---
id: RR-VGS0D
type: review-response
title: gh issue list --search tokenizes unquoted title — dedup can false-match
finding: 'security.yml:69 passes the title unquoted to gh issue list --search, so GitHub tokenizes the words and also interprets the leading `govulncheck:` as a bogus qualifier. Unrelated issues could match; real dupes could miss. Fix: quote the title as a phrase (`--search "\"$TITLE\" in:title"`) and drop the colon.'
severity: critical
resolution: security.yml now uses `--search "\"$TITLE\" in:title"` (quoted phrase) plus an explicit `jq` title-equality filter, and the title no longer contains a colon. Dedup matches the exact title.
status: addressed
---
