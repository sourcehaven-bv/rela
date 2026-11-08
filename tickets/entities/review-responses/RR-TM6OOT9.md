---
id: RR-TM6OOT9
type: review-response
title: Raw MemorySender bypasses the mail RecipientPolicy and the outbox is async
finding: mail/script.go:440-453 wraps the transport in NewLuaSender with RecipientPolicy; appbuild/mail.go:76-81 delivers in a goroutine.
severity: significant
resolution: Keep parsing mail.yaml and replace only the transport via SenderFor with the shared MemorySender so policy applies. Outbox delivery in the test world is flushed synchronously before t.mail() returns.
status: addressed
---
