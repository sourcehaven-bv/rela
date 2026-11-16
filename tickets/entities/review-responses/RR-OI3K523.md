---
id: RR-OI3K523
type: review-response
title: Top-level merge key hides a misspelled block
finding: 'The key walk read only direct top-level keys, so <<: {remote-mcp: ...} loaded cleanly and dropped enabled: false.'
severity: significant
resolution: refuseMisspelledKeys follows << merges (mapping or sequence of aliases); tests pin both forms.
status: addressed
---
