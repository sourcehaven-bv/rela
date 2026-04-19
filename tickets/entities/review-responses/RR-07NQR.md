---
id: RR-07NQR
type: review-response
title: cryptofs.Stat returns ciphertext size; decision deserves explicit doc
finding: cryptofs.go:86-89 documents Stat returns ciphertext metadata. Good. But see RR-HBER2 — consumer (fsstore.loadAttachmentsIndex) treats that as plaintext. Decorator is contract-clean; consumer buggy. Stricter design would return plaintext size from Stat (reading to measure), paying CPU to eliminate foot-gun. Current split-responsibility is clean but unfriendly. Worth explicit decision doc.
severity: minor
reason: Consumer (loadAttachmentsIndex) was the real bug and has been fixed under RR-HBER2 — it now reads via bytes.ReadFile to measure plaintext. cryptofs.Stat returning ciphertext metadata is the correct contract for a transparent decorator (Stat should describe what's on disk, not what would be readable). Making Stat decrypt-to-measure would pay CPU cost across every attachment listing, for a contract ambiguity that no longer has a buggy consumer.
status: wont-fix
---
