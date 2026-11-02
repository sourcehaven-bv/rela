---
id: RR-X1B3SD
type: review-response
title: REST docs point clients at a can_post field the REST wire does not have
finding: internal/apiwire/v1/channels.go:523, internal/openapi/channels.go:100-102 (served in the OpenAPI goldens) and docs-project/entities/guides/GUIDE-realtime.md:2488 all say the channel's "_actions.post and can_post" stay authoritative. can_post exists only on realtime.MessageTypeView (internal/realtime/read.go:91), which MCP read_channel and the CLI serve; no type in internal/apiwire/v1 or frontend/src/types/channels.ts has can_post. A REST client author reading the OpenAPI description will look for a field that is not there. On the REST surfaces name only _actions.post (the per-channel affordance REST does serve); keep "can_post" in the MCP read_channel text and on MessageTypeView, where it is real. Regenerate the OpenAPI goldens and docs/ after the edit.
severity: minor
resolution: "addressed\tThe apiwire godoc, the OpenAPI description of in and the guide's _schema/channels paragraph now name only _actions.post; can_post stays in the MCP read_channel text and on MessageTypeView. OpenAPI goldens and docs/realtime.md regenerated."
status: addressed
---
