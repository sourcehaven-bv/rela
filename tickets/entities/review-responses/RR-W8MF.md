---
id: RR-W8MF
type: review-response
title: Test name uses em-dash; reduces greppability
finding: '''markdown editor bundles Font Awesome — no maxcdn CDN fetch'' uses an em-dash; rest of the file uses ASCII names. Replace with a colon or hyphen for grep-friendliness.'
severity: nit
resolution: Renamed to 'markdown editor bundles Font Awesome (no CDN fetch)' — parentheses, no em-dash. Greppable with `grep 'Font Awesome'`.
status: addressed
---
