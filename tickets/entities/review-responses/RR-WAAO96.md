---
id: RR-WAAO96
type: review-response
title: OpenAPI post body omits required props
finding: internal/openapi/channels.go does not mark props required when the message type has required properties.
severity: nit
resolution: The post body schema marks props required when the message type has required properties.
status: addressed
---
