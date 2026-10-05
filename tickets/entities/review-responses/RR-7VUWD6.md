---
id: RR-7VUWD6
type: review-response
title: '[security] Upload suffix reveals another face''s file name'
finding: Names are unique per entity, so an upload colliding with another face's file is suffixed. A writer learns a file of that name exists on some face.
severity: minor
resolution: 'Accepted one-bit channel: the writer cannot list or read the file. Documented in the service model comment and docs/acl-security.md.'
status: addressed
---
