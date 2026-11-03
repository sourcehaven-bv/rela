---
id: RR-O350KT
type: review-response
title: Nothing stops the bare-string allowlist from growing
finding: A new file could be added to bare-strings.json with any count.
severity: minor
resolution: 'Added a ceiling test: listed files <= 111 and total strings <= 929, the starting size.'
status: addressed
---
