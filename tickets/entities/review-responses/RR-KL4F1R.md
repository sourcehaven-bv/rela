---
id: RR-KL4F1R
type: review-response
title: Export uses a different read path from the panel
finding: PolicyReader.Filter needs full entities and ignores the world, so export's 'readable' set could differ from the panel, and combining it with ResolveHeaders would redact twice.
severity: significant
resolution: 'Plan: one helper readableItems(ctx, pile) built on the batched header resolution feeds panel, counts, scope and export. Export renders id/type/title from the redacted headers; Filter is not used on this path; redaction happens once.'
status: addressed
---
