---
id: RR-VYP003
type: review-response
title: 'Downgrade keeps a stamped index'
finding: 'An older build opening a stamped index keeps the stamp and may write bare keys, which a later upgrade would trust.'
severity: minor
resolution: 'Documented on indexFormat: switching between versions needs the index directory removed.'
reason: 'Older builds cannot be changed; the case is rare.'
status: addressed
---
