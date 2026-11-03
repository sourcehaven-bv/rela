---
id: RR-VAUS4P
type: review-response
title: OpenAPI still states the old count rule
finding: 'internal/openapi/channels.go:644-645 describes read.unread as "Removed messages and session notices do not count". The godoc in internal/apiwire/v1/channels.go and frontend/src/types/channels.ts was updated, but the served OpenAPI text was not, so API clients get an incomplete rule. Add "and messages of a type with unread: false" and regenerate the goldens. Q6 rules out serving the key, not describing the rule correctly.'
severity: minor
resolution: 'ADDRESSED internal/openapi/channels.go read.unread now adds messages of a type with unread: false. No golden holds this text.'
status: addressed
---
