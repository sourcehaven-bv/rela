---
id: RR-UY0WDX
type: review-response
title: Restricted-ref edit rule is affordance-only
finding: 'internal/realtime/msgops.go:83 leaves edit out of _actions when a ref prop renders restricted, and the apiwire doc (internal/apiwire/v1/channels.go:115-119) says every operation is authorized again. Service.Edit (internal/realtime/ops.go:47-103) does not apply the same rule: an author whose client ignores _actions can PATCH the message, and because EditMessageRequest replaces every prop, the stored ref the author can no longer read is silently dropped. Impact is limited to the author''s own message (no confidentiality loss, no other user''s data), so this is an integrity/defense-in-depth gap rather than an exploit; the plan (t7-plan.md R7) accepts no server-side merge in T7. Fix: in Service.Edit, after s.message(c, seq), refuse with 409 (e.g. a new TypeEditRestrictedRef) when a stored ref prop of m is not readable by c and the new props do not name the same id, mirroring hasRestrictedRef; or merge unnamed ref props server-side. Either way the affordance and the operation then agree, as the msgops.go godoc claims.'
severity: minor
resolution: The server now keeps unreadable unnamed refs on edit, so the rule no longer depends on the affordance, and Edit and the affordance share editCheck.
status: addressed
---
