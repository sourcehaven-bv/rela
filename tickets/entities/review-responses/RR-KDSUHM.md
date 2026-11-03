---
id: RR-KDSUHM
type: review-response
title: A read or field ceiling can make the find miss and create duplicates
finding: If the find type is unreadable or a match property is hidden, create_if_missing creates on every delivery.
severity: significant
resolution: 'acl map --webhook adds a webhook check: read on the find type (at find.face), create, update, and find.match properties the field ceiling hides. --expect exits 1 when the check fails.'
status: addressed
---
