---
id: RR-8HSQ
type: review-response
title: No typed errors or rate-limit handling; scripts must string-match
finding: 'Plan returns raw err.Error() to Lua as a string. There is no special-casing of HTTP 429, no Retry-After parsing, no error taxonomy. Real OpenAI-compat endpoints return 429 constantly; scripts have no clean way to retry without parsing prose error messages. Even if retries are deferred, the typed error surface must exist now — once strings are returned, the API is locked in. Fix: define an error taxonomy with stable kinds (rate_limit, auth, timeout, upstream, network, bad_response, bad_request, server_error). Surface to Lua as a table {kind, status, message, retry_after} via the second return. This is the leverage opportunity #18 reframed as a critical: the cost of NOT doing this now is much higher than the cost of doing it.'
severity: critical
resolution: 'Introduced internal/ai/errors.go with ErrKind enum (not_configured, auth, bad_request, rate_limited, server_error, timeout, network, bad_response, streaming_unsupported) and Error struct {Kind, Status, Message, RetryAfter, cause}. classify() maps HTTP responses to typed errors. Lua binding marshals to a table {kind, status, message, retry_after} as the second return. ACs #9-14, #28, #35 cover the typed error surface. Retries themselves are deferred but the rate_limited error includes RetryAfter so scripts can implement backoff.'
status: addressed
---
