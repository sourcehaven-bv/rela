---
id: RR-G2L5P
type: review-response
title: 'Acceptance criterion #1 is self-contradictory'
finding: 'AC1 says grep for encryption/crypto/Seal/Unseal in internal/store/fsstore returns zero hits AND ''outside the watcher (single Unseal call)''. Those are not the same statement. Reviewer running grep will get hits in watcher.go and the criterion reads as both pass and fail. AC1 as stated also bans the recentHashes callback wiring needed to fix finding #1.'
severity: critical
resolution: AC#1 rewritten to enumerate the specific files that must be crypto-free (fsstore.go, markdown.go, attachment.go, index.go, formatter.go) and explicitly allow watcher.go's IsCorrupted import only. No more ambiguous 'grep returns zero hits' self-contradiction.
status: addressed
---
