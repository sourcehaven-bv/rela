---
id: RR-8624OST
type: review-response
title: Document scripts can act outside the store
finding: Piles, attachments, files, ai/http/mail and rela.history bypass the store seam under an instant.
severity: significant
resolution: Those bindings raise an error under an instant (internal/lua/instant.go); pinned in TestAsOf_SQLite_DocumentReadsTheInstant.
status: addressed
---
