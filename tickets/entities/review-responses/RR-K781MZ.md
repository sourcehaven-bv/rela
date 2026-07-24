---
id: RR-K781MZ
type: review-response
title: Split flush-on-author-change out of v1 — columns + sweep-stamping fully fix the reported symptom
finding: Migration + Attribution carrier + pgstore stamping + sweep fallback completely solve the user-visible bug ('unknown · version-sweep' on ordinary edits). Flush-on-author-change only addresses the narrower case of two DIFFERENT authors editing the same entity within one debounce window (default 5m), and it carries every hard correctness concern found in this review (advisory-lock interaction, op-choice, purge tombstone interaction, dedup uniqueness). Recommend shipping attribution first and landing flush as a follow-up ticket with its semantics designed and tested in isolation.
severity: significant
resolution: 'Split executed: TKT-ZIRMGM now ships migration + store.Attribution carrier + pgstore stamping + sweep fallback (fully fixes the reported ''unknown · version-sweep'' symptom). Flush-on-author-change moved to follow-up TKT-0IGI4V with all pinned semantics from this review as requirements.'
status: addressed
---
