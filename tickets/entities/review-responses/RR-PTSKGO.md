---
id: RR-PTSKGO
type: review-response
title: Lifecycle request body schema admits fields the server rejects
finding: 'internal/openapi/channels.go:204 uses ObjectSchema(map{}, nil) without `additionalProperties: false`, but runLifecycle (channels_handler.go:357) decodes strictly into struct{} and answers 400 to any field. Declare the closed object so generated clients match.'
severity: nit
resolution: 'The lifecycle body schema now sets additionalProperties to {"not": {}}, JSON Schema''s false schema, through a new Schema.Not field. The test asserts it.'
status: addressed
---
