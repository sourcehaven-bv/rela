---
id: RR-WQKEKG
type: review-response
title: '[security] Base-name comparison lets an echo rewrite the stored path'
finding: The file rule compared base names, so an echo could store a different path with the same base name.
severity: minor
resolution: 'Same fix as the echo finding: the stored value is kept (pinStoredFileValues).'
status: addressed
---
