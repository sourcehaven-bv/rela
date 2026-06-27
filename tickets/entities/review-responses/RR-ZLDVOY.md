---
id: RR-ZLDVOY
type: review-response
title: AI timeout handler's ctx branch was dead code — comment claimed a win it didn't deliver
finding: 'Reviewer instrumented it: the AI test''s client gives up via a 50ms context deadline, which abandons the request but leaves the keep-alive socket open — r.Context() never cancels, so the select arm was unreachable and server.Close still waited out the timer (verified with a 30s timer: blocked 29.95s). The lua test''s mechanism (client Timeout) genuinely closes the connection and was fine.'
severity: significant
resolution: Cleanup now calls server.CloseClientConnections() before Close — force-closing cancels in-flight request contexts, making the select branch live regardless of how the client gave up. Handler comment rewritten to state the actual mechanics. Timing verified.
status: addressed
---
