---
id: RR-G6QRRI
type: review-response
title: Cascade count repeated on every per-face audit record
finding: Each face's delete record carried the full cascade count, which reads as that many relations per face.
severity: minor
resolution: recordFamilyDeleteAudit puts the cascade count on the first record only.
status: addressed
---
