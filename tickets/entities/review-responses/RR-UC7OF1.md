---
id: RR-UC7OF1
type: review-response
title: Message for a moved read says not to add a file
finding: Moving a function to a new file reports a new read, and the advice did not cover moving the count with it.
severity: minor
resolution: 'Every growth message now ends with: if you only moved an existing read between files, move its allowlist count with it.'
status: addressed
---
