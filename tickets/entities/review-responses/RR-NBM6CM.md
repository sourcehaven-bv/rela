---
id: RR-NBM6CM
type: review-response
title: jsonschema.go is not a config schema
finding: internal/realtimecfg/jsonschema.go builds MessageType.PropsSchema, the JSON schema of a message's properties. It does not list realtime.yaml keys, so the step "add in to jsonschema.go if it lists message-type keys" does nothing; drop it. Consider a refusedKeys entry in keys.go for the likely wrong spellings (for example channel_types.*.message_types.*.states and channel_types.*.message_types.*.from) that points to in:.
severity: nit
resolution: The `jsonschema.go` step is dropped, and `refusedKeys` gains `message_types.*.states` and `message_types.*.from`, both pointing to `in:`.
status: addressed
---
