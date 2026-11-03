---
id: RR-3WYBDG
type: review-response
title: limit=0 is the default, not a 400
finding: 'internal/realtime/inbox.go:174 maps 0 to 50. The REST handler, the CLI (`--limit 0`) and the OpenAPI text ("limit is not a whole number from 1 to 200") present 1 to 200 as the valid range, and the plan''s input validation says that anything else is 400. TestInboxHandler_Errors omits `?limit=0`. Fix: have the REST and CLI layers distinguish "absent" from 0, or document that 0 means the default.'
severity: nit
resolution: limit=0 is rejected on REST and CLI via realtime.CheckInboxLimit; MCP already refused it; tests include 0
status: addressed
---
