---
id: RR-MFSL0KA
type: review-response
title: Pre-check and output gate miss an answer closing its question
finding: precheckOutputs runs Decide on the pre-lock head; it never sees the answer close its question, so the last answer predicts no output and the gated transition silently does not move. Batch messages 2..N are prechecked against the pre-batch head.
severity: critical
resolution: 'Plan: question tracking is a pure function applyBatch(head, msgs) used by both the pre-check and the store; the pre-check simulates the whole batch on a copy of the head. Tests: answer transition with outputs and the open_questions guard; batch whose moving message has outputs.'
status: addressed
---
