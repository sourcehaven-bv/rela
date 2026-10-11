---
id: RR-A0UQEN
type: review-response
title: Cap boundaries and truncated path untested
finding: No case at exactly 200, none across pages ending on the cap, no page.Truncated test, and no multi-page budget.
severity: nit
resolution: Added "exactly the cap" and "exactly the cap across pages" cases. The Truncated branch reuses the resolver's flag, which TKT-Q6AKUN's resolver tests pin; the per-page cost is fixed and bounded by mentionStepPages, so no multi-page budget test was added.
status: addressed
---
