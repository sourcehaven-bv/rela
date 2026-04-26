---
id: RR-8LQ3Q
type: review-response
title: 'F7: captured print() output is itself a leak vector'
finding: 'Plan redacts args by key and truncates large strings, but does not redact captured stdout. A script that does print(rela.secrets.api_key) then errors lands the secret in captured_output. Combined with F2 (unauthenticated server), this is a real read-secrets-via-error gadget. Plan needs an explicit policy: same redaction applied to captured_output, OR project-config gate, OR scrub recognized secret-shaped values (JWT, long random hex/base64) from print output.'
severity: significant
resolution: 'Same redactValue/redactString applied to captured output line-by-line via redactCapturedOutput; capped at 16KB. Additionally gated by loopback config (F2): non-loopback default omits captured_output entirely.'
status: addressed
---
