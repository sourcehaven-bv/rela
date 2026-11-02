---
id: RR-IQ99D0E
type: review-response
title: Forged hints and gaps can cause replay storms
finding: A resync replays every subscription; anyone who can append to the journal or NOTIFY can force it repeatedly. NOTIFY exposes principal keys to every listener on the database.
severity: significant
resolution: Gap and resync hints limited to one resync per 5 s per Local; counted and logged. Exposure documented in docs/postgres-backend.md (hashing with a public salt gives no protection).
status: addressed
---
