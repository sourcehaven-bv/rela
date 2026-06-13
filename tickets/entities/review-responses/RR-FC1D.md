---
id: RR-FC1D
type: review-response
title: 'S4 + S5 + L4: AutoSaveIndicator placement and per-row instance cost'
finding: |
  - S4: 50 SectionEditForm instances ≈ 250 reactive refs + 50 watchers. Defensible at the page level but not zero. Plan mentions a perf smoke test but doesn't commit to a threshold.
  - S5: SectionEditForm's current absolute-positioned `.section-edit-form-indicator` won't visually work inside a card or list-row. CSS-only adjustment is insufficient because the template puts the indicator inside the form's own root div.
  - L4: A slot for the indicator would let the host decide placement.
severity: significant
status: addressed
resolution: |
  - S4: PLAN risks section commits to a soft cap of 100 rows per cards/list section before falling back to display-mode for that section. Above 100 rows the user is more likely to be browsing than editing anyway. Add a smoke test asserting 100-row mount completes within 200ms wall-clock.
  - S5 + L4 ADOPTED: SectionEditForm gains a named slot `<slot name="indicator"><AutoSaveIndicator .../></slot>` (default preserves current behaviour). Host overrides per call site:
    - Entry-section call site: omit override → default placement preserved (no regression).
    - Cards call site: override puts the indicator inside the card header alongside the edit button.
    - List call site: override puts the indicator inline-right of the row title.

  PLAN AC 6 amended: indicator placement is host-controlled via the slot. SectionEditForm's API gains a slot (compatible with all existing callers since the default preserves current shape).
---
