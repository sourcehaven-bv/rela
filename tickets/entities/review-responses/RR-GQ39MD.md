---
id: RR-GQ39MD
type: review-response
title: Delete 204 response is built by creating a 200 and deleting it
finding: internal/openapi/channels.go:189-192 calls channelResponses("200", "", nil, ...) and then deletes "200". Build the error map and add 204 with the existing `withResponse` helper.
severity: nit
resolution: Split out channelErrors(errs...). channelResponses now uses it, and the delete path builds withResponse(channelErrors(...), "204", ...).
status: addressed
---
