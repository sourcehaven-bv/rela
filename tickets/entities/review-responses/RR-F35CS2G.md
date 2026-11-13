---
id: RR-F35CS2G
type: review-response
title: Consecutive seqs criterion cannot pass
finding: A session move inserts a notice after the moving message, so batch seqs are not consecutive.
severity: minor
resolution: 'Plan: criterion reads ascending seqs, notices may sit between; Batch is set only on posted messages.'
status: addressed
---
