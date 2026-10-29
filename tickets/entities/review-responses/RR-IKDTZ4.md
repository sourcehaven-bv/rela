---
id: RR-IKDTZ4
type: review-response
title: 400 for a misused expect_generation comes after 409s and is not charged
finding: 'internal/realtime/service.go:399: an expect_generation on a no-session or ephemeral type is reported only after requireOpen and the in: pre-check, so a closed channel answers 409 channel_not_open and an ephemeral type via CLI/MCP answers signals_not_kept first. That is acceptable, but Post''s doc says ''an invalid message is [charged a token]'', and this 400 is not. Mention the exception in the doc comment, or move the shape check (negative, no session, ephemeral) to right after the message-type lookup, where the other request-shape 400 lives.'
severity: nit
resolution: 'Moved the expect_generation shape checks (negative, no session, ephemeral) to right after the message-type lookup in Post, kept the generation 409 after the in: check, and updated Post''s doc to say these 400s cost no token.'
status: addressed
---
