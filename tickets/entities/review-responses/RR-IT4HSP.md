---
id: RR-IT4HSP
type: review-response
title: Candidate fetch cost scales with collection size to solve a linked-edge-sized problem
finding: 'The picker now fetches up to 50 sequential pages per target type, and loadCandidates loops target types sequentially too, so three target types at 5,000 entities each is ~22s before the picker is usable. That cost is paid to resolve the type of typically 1-5 already-linked IDs. The root conflation is that `candidates` serves two purposes with opposite cost profiles: dropdown options (want a bounded, searchable set) and an ID->type lookup (wants exactly the linked IDs).'
severity: significant
reason: Deferred to TKT-MG2FXQ, not done in this bug fix. The current change is correct and is a no-op for typical projects (one page, one request, stops). Restructuring the picker onto server-side search plus a targeted resolve is a redesign of the widget, not a bug fix, and bundling it here would make the regression surface far larger than the defect. The reviewer explicitly asked for a ticket rather than code.
status: deferred
---
