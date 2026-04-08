---
id: RR-TH21
type: review-response
title: Streamed response from server expecting non-streamed produces confusing error
finding: 'Plan does not explicitly send ''stream: false'' in the request body. Some compat servers (older Ollama, some gateways) SSE-stream by default. The JSON decoder will see ''data: {...}'' and produce a confusing parse error. Fix: explicitly set stream: false in the request body. If response Content-Type is text/event-stream, return a clear typed error like ''streaming responses are not supported in this slice'' rather than a JSON parse error.'
severity: significant
resolution: 'Request body now always sends ''stream'': false (AC #16). Response Content-Type validation rejects text/event-stream with ErrStreamingUnsupported (AC #17). Users who want streaming get a clear typed error rather than a confusing JSON parse failure.'
status: addressed
---
