---
id: RR-GNEITSI
type: review-response
title: Warning repeats on every load
finding: The CLI loads the schema on every command and the server on every reload, so the warning repeats.
severity: minor
resolution: 'Intended: one warning per file per load is the nudge to run rela migrate. Kept to a single line per file.'
status: addressed
---
