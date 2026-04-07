---
id: RR-QIEC
type: review-response
title: No operational logging at all; debugging will be miserable
finding: 'Plan defers ''audit logging'' (every prompt/response stored — fine to defer) but adds zero operational logging. When users report ''AI calls don''t work'', there is nothing to look at — no request URL, no status code, no latency, no token count. This is table-stakes operational visibility, not an audit feature. Fix: add operational logging requirements to the plan. At minimum, using whatever logger the rest of the codebase uses (find during impl): debug log on request start (URL + model + message count, no headers, no content); info log on success (status + latency + token count); warn log on non-2xx with status + truncated body. Explicitly: zero PII, zero secrets, zero prompt content.'
severity: significant
resolution: 'Added operational logging requirements using stdlib log (matching internal/workspace/workspace.go). Levels: DEBUG (request start with base_url + model + message_count, no headers/content/key), INFO (success with status + model + latency_ms + token counts), WARN (HTTP/network errors with status + kind + redacted body snippet). ACs #38 and #39 verify log lines and that the API key sentinel never appears in captured log output. Audit logging (storing prompts/responses) remains deferred.'
status: addressed
---
