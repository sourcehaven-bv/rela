---
id: RR-LBHZL
type: review-response
title: image_inline constructor not exposed (asymmetric with link_inline)
finding: rela.md.link_inline exposes a link constructor but no rela.md.image_inline. Scripts wanting to construct images hand-build the table.
severity: nit
reason: Real demand unclear; defer to a small follow-up if a user asks. Trivial to add later.
status: deferred
---
