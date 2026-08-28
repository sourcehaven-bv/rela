---
id: RR-DR-SHELLTRIPWIRE
type: review-response
title: Keeping the 4-variant shell model is correct, but the boundary that keeps it correct is unwritten
finding: 'The plan''s reasoning is right: the variant count is a product of how many INJECTABLE entry
  points exist (two, per decision 1), not of how many files the folder holds. Arbitrary assets are never
  injected so they never multiply the count; 2^2 = 4 holds. What is missing is the trip-wire: if anyone
  adds a third injected entry (custom.head.html, a second stylesheet, a per-theme variant) the model goes
  to 8 and selectShell''s switch becomes combinatorial. Nothing currently warns them.'
severity: minor
status: addressed
resolution: Folded into TKT-IWMETE and PLAN-6VVJJZ before implementation. Note in the shellVariants godoc
  that the variant table is only viable while the injected set is exactly two and fixed, so the next feature
  does not quietly add a fifth field without noticing the design has outgrown it.
---

Raised by `/design-review` of TKT-IWMETE before implementation.
