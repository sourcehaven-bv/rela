---
id: RR-TTNT2
type: review-response
title: Acknowledge html.WithUnsafe + multi-entity content as broader attack surface
finding: Plan calls current html.WithUnsafe + DOMPurify combo 'not a regression.' Technically correct, but Lua docs pull arbitrary entity.content markdown (via rela.list_entities etc.) into the output, broadening paths by which user-submitted content reaches the unsafe markdown stream. Still OK end-to-end via DOMPurify — but the security section wording is too confident.
severity: minor
resolution: Security section and AC-DOC1 now explicitly name DOMPurify as the sole sanitization boundary and flag future HTML consumers (PDF export, copy-HTML) as needing their own.
status: addressed
---

From design-review on PLAN-78HJO.

Plan should acknowledge: "frontend sanitization remains the only boundary.
Future consumers of the rendered HTML (PDF export, copy-HTML button) would
inherit the risk and need their own sanitization layer."
