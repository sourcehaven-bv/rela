---
id: TKT-F2D5U5
type: ticket
title: 'Demo: one rela config exercising BOTH axes — ISMS policy lifecycle + multilingual blog posts'
kind: enhancement
priority: high
effort: l
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

---

## SCOPE SET BY JEROEN 2026-08-23: both scenarios in ONE config

A single axis would not prove the primitive generalises, so the demo carries
two entity types on two different axes:

- **`policy`** — the STAGE axis (`draft` → `published`). A lifecycle:
  one-way, and the draft is CONSUMED by promotion. Published is not directly
  editable; it changes only via the `promote-policy` copy definition.
- **`blog-post`** — the LANGUAGE axis (`en`, `nl`, `fr`). Peers, not a
  lifecycle: any-to-any, the source SURVIVES, several targets exist at once.

**This does NOT require multi-axis** (design doc §11, deliberately not built
in v1). Multi-axis means ONE entity holding a combined coordinate
(`nl+draft`); here each TYPE uses ONE axis, which is exactly what shipped.
Do not attempt `for_each` world templates either (§4.5, tentative) — declare
the worlds concretely.

## What the demo must show

The same UI primitive rendering two ways, driven by which copy definitions
name the currently-viewed face as their source:

- Published policy: ONE applicable definition -> a single **"go to draft"**
  button (operator-configurable text).
- English blog post: THREE applicable definitions -> a **menu**, one entry
  per language.

Plus: the copy succeeding with a message and navigation to the result; the
"other faces exist" indicator (one entry for policy, a list for blog-post);
and a published-world reader genuinely unable to see drafts.

## UX decisions — made by Jeroen, recorded as RULING 9

Implement them; do not re-decide them. Promote is a button on the draft; no
permission -> no button; a published face has NO edit button but carries a
"go to draft" action, hidden when the draft is inaccessible; promote
confirms by default (operator-configurable); success = message + navigate;
creating an absent face is an EXPLICIT TARGETED action, never a silent
auto-fork; faces the viewer may not read are OMITTED ENTIRELY.

## Backend prerequisite (ships BEFORE this ticket starts)

`after: discard | keep` per copy definition, default `keep` — the ISMS draft
is consumed by promotion, the English blog post is not. Small: the kernel
already runs in one `store.Tx`, so `discard` is a delete inside it, never a
second operation that can half-fail. Follow-up PR on the Step-4 stack, so
this ticket is pure SPA work over a settled backend.

## Known limitation to handle honestly

A world-bound surface cannot carry search until per-world indexing
(TKT-9KZGJO, Step 5). Either omit search on that surface or label it — do
not let it silently return default-world hits.
