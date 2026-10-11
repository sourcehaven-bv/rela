---
id: RR-9NTK09
type: review-response
title: _attachments declared only for types with file properties
finding: The server sends _attachments ({}) on every per-entity response but the spec declared it only for types with file properties; a test pinned the wrong behaviour.
severity: significant
resolution: _attachments is declared on every entity schema; the test asserting its absence was removed.
status: addressed
---
