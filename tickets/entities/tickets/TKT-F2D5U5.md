---
id: TKT-F2D5U5
type: ticket
title: 'ISMS demo: draft/published policy lifecycle with promote, end to end in the UI'
kind: enhancement
priority: high
effort: m
status: backlog
---

The first real user-facing demonstration of FEAT-9CD2MX (content states and
worlds). Requested 2026-08-23.

**Scenario:** a minimal ISMS project (3-5 `policy` entities) where:

- A **published** policy is served to readers and is **NOT directly
editable** — no role holds `update` on `policy@published`; it changes only via
the promote copy definition (design doc §9.2).
- A **draft** face is editable by authors and invisible to published-world
readers (existence in a world IS the publication bit, §4.1).
- **Promote** copies draft → published as an audited, guarded operation
(§9.1), and the published world immediately serves the new face.

**Gated on Steps 3 and 4 — this cannot be demonstrated honestly before them:**
- TKT-DN37J2 (Step 3) makes `policy@published` unwritable and introduces
request-level world selection with its grant check. Without it, an editor can
simply write the published state and the demo's central property is a claim
rather than a fact.
- TKT-C1XUA8 (Step 4) provides the copy kernel and the declared `promote`
definition. Without it, "promotion" means hand-editing the published file.

**SPA work this ticket owns** (not covered by Steps 3/4, which are backend):
- A world selector on editor surfaces, driven by the Step-3 grant so a
principal only sees worlds they may read.
- A promote affordance on a draft, routed through the existing `_actions`
machinery (UI hint, re-authorized server-side per §9.3 rule 1).
- Showing provenance: which face is displayed and whether it arrived by
fallback (§4.2 guard rule 2) — a reader must be able to tell "published" from
"default, because nothing is published yet".
- Handling the Step-2 limitation honestly: a world-bound surface cannot
carry search until per-world indexing (Step 5, TKT-9KZGJO). Either omit search
on that surface or label it.

**Explicitly minimal on content:** a handful of policies, enough to show
resolution and promotion. Not a full ISO 27001 control set.
