---
id: RR-B6TUJ2L
type: review-response
title: assertTarget compared bare ids only
finding: A snapshot or refetch of another face of the same entity passed the identity check.
severity: minor
resolution: 'A faced target now also requires the response''s _self face to match. A bare target still accepts any face, because the server picks it. Test: ''accepts its own faced row and refuses another face of the same entity''.'
status: addressed
---
