---
id: RR-J3KA9
type: review-response
title: Document in AC8 that SSE live-reload only covers the entry entity
finding: AC8 currently says 'SSE live reload' without qualification. For Lua docs composing from multiple entities, only the entry entity change triggers reload (since computeDocumentHash only hashes the entry). Users will expect multi-entity composition to live-reload. Guide must spell out the limitation.
severity: significant
resolution: AC-DOC1 requires the guide to state the entry-only SSE limitation explicitly; refresh-button escape hatch documented.
status: addressed
---

From design-review on PLAN-78HJO. Not a regression — command: docs are already
entry-only — but AC8 oversells. Fix wording in AC8 and add an explicit caveat in
the Documents guide section.
