---
id: RR-6J80ET
type: review-response
title: Delete wake only reaches the local worker; multi-process pg purge waits up to 1h
finding: appbuild/channels.go:158,173 ping the process-local confirmer channel. When the deleting process is not the leader, nothing reaches the leader until its hourly tick, contradicting docs/realtime.md "also right after the delete". Either signal through the database (NOTIFY or a due-time poll) or fix the docs.
severity: significant
resolution: A minute purge poll over DueTombstones runs in whichever process gets the try-lock, and the docs state the timing exactly.
status: addressed
---
