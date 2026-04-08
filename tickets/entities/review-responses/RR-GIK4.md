---
id: RR-GIK4
type: review-response
title: Content-Type validation missing; HTML error pages decode to confusing JSON errors
finding: 'Plan caps response body at 10 MiB (good for OOM) but does not validate Content-Type before decoding. An HTML error page from a misconfigured corporate proxy decodes to ''unexpected character <'' and the user has no idea their proxy is intercepting traffic. Fix: check Content-Type is application/json (or includes ''json'') before decoding. On mismatch, return an error including upstream status + first 200 bytes of body so the user can diagnose. Apply this to all decode error paths, not just JSON-parse failures.'
severity: significant
resolution: 'Provider.Chat now validates Content-Type before JSON decoding. text/event-stream → ErrStreamingUnsupported. Anything else not containing ''json'' → ErrBadResponse with status + first 200 bytes of body. AC #18 tests HTML responses; AC #23 covers malformed JSON with body snippet. Body snippet is key-redacted via redactKey.'
status: addressed
---
