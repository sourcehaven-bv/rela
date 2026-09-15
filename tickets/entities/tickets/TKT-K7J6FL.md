---
id: TKT-K7J6FL
type: ticket
title: 'export: documents export via transforms, like entity and list views'
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

The `transforms:` registry (FEAT-5IUVGX) was designed so that **every**
markdown-producing surface gains every registered format automatically. Two of
the three renderers it names are wired up:

| Surface | Endpoint | Status |
|---|---|---|
| Entity view | `GET /api/v1/{plural}/{id}/_export` | shipped (TKT-JF5JI8) |
| List view | `GET /api/v1/{plural}/_export` | shipped (TKT-JF5JI8, TKT-95XU13) |
| **Lua document** | — | **missing** |

This is the gap. A document is *already* a markdown producer — the render
pipeline is markdown → HTML, and `documentService.RenderMarkdown` exists and is
exported precisely because export needs the pre-HTML step. Yet the richest
authored surface in the product (an operator-written report) is the only one a
user cannot get as a PDF.

Documents are also the surface most likely to want one. An entity export is a
property table; a list export is a column table. A document is a hand-authored
report — a release note, a sales summary, a board pack. That is the artifact
people actually mail around.

`internal/transform`'s own package doc states the same intent independently: "an
entity view, a list view, a Lua document — gains that format for free". Two
design documents named this surface; only the code was missing.

## Scope

**In scope**

- Export for **entity-anchored** documents (`/_documents/{doc}/{entityId}/_export`).
- Export for **standalone** documents (`/_documents/{doc}/_export`).
- The SPA `ExportMenu` in `DocumentView.vue` (the component is already generic —
it takes a `urlFor` callback, so this is wiring, not new UI).
- Extracting the render handlers' ordered gate chain into one shared helper used
by both the render and the export path.
- **Closing the entity-existence oracle in the anchored document handler**
(RR-XIYP3I) — see below.

**Out of scope**

- `command:` renderers. Refused; a *policy* choice rather than a structural
impossibility (see RR-T5POTJ), so the guard lives in both the render layer and
the handler.
- Async export for slow converters — a documented v1 limit shared with the two
existing export surfaces.
- A `documents.<id>.export_render` override. A document **is** the render.
- An `exportable:` opt-out flag — export inherits `permission:` and reveals
nothing the on-screen render does not.
- Caching/singleflight of export output.
- Field-level redaction at the export boundary. A document's Lua already reads
through `lua.ReadDeps.VisibleReader`; re-redacting would violate
`visibility.Redact`'s non-composability contract (RR-MZE4IA).

## Scope addition from design review (RR-XIYP3I)

The anchored document handler has a **pre-existing entity-existence oracle**:
`api_v1.go:2591` reads the store raw and 404s *before* `gateReadOrNotFound`,
with a different error code, a different title, and the entity id echoed back.

| Probe | Code | Title |
|---|---|---|
| id does not exist | `entity_not_found` | `entity "TKT-999" not found` |
| id exists, read denied | `not_found` | `Entity not found` |

That violates `entityNotFoundTitle`'s own godoc ("Any handler that 404s on the
read path MUST use this const, not a fresh literal, or the bodies drift and
existence leaks", RR-NGMI), and it is the only read-path 404 in the package that
interpolates an entity id.

Fixed **here** rather than split out, by explicit decision: this ticket extracts
precisely that chain into a shared helper, so leaving it would ship a known
oracle on a second route, and any split would land the fix in this diff anyway.

## Security notes

Export sits downstream of the *same* gates as the HTML render, in the same
order, with no new decision of its own. Post-review the order is:

1. `isSafePathSegment` on both path segments.
2. Document config lookup; kind-mismatch rejection (anchored vs standalone).
3. `gateReadOrNotFound` on the document's `entity_type` (anchored only) —
**now above the store read**, so a hidden id and a nonexistent id are
indistinguishable in body and in timing.
4. `gateDocumentPermission`.
5. `gateElevatedDocument` — the closed switch on the ACL implementation.
**Load-bearing:** an elevated document reads through a raw handle, so this gate
is the only boundary. An export path that skipped it would turn a
permission-gated report into an ungated download.
6. Entity type mismatch — *below* the read gate, so a denied principal never
gets a 400 type oracle.

Then `RenderMarkdown` (anchored) or `RenderStandaloneMarkdown` (standalone),
then the shared `transform.Engine`, then `writeExportResponse`.

Properties that are easy to lose and are therefore pinned by tests:

- **The engine stays the shared one.** It owns the bounded worker pool capping
concurrent converters; a second engine would bound nothing.
- **Export never calls `GetCached`.** That key omits *both* ConfigID and
principal (unlike `Render`'s singleflight key), so reading it from a
per-principal path would reintroduce RR-2QSGLU.
- **Script errors stay a flat 500.** Matching both shipped export routes;
`lua.ScriptError` detail (`Source`/`Stack`/`CapturedOutput`) must not reach an
export error body, since for an elevated document that is content rendered under
a bypassed ACL (RR-74LHU1).
- **`RenderStandaloneMarkdown` keeps `elevatedDeps`** — otherwise a standalone
export silently loses elevation and declared capabilities (RR-T5POTJ).

## Acceptance criteria

1. `GET /api/v1/_documents/{doc}/{entityId}/_export?transform=X` returns the
converted bytes with hardened download headers.
2. `GET /api/v1/_documents/{doc}/_export?transform=X` does the same for a
standalone document, with `rela.document.entry_id` nil (not `""`).
3. Every gate above is enforced on the export route, verified per gate.
4. An unknown transform is a 404; a missing `?transform=` is a 400.
5. A `command:`-rendered document is refused with a clear error, not a partial
or entity-less render.
6. `DocumentView.vue` shows the Export menu when transforms are registered, and
hides it when none are.
7. A document named `_export` is a config-load error.
8. **Hidden and absent entity ids produce byte-identical 404 bodies (minus
`instance`) on both the render and the export route** (RR-XIYP3I).
9. A raising script on the export route yields a 500 carrying none of `Source`,
`Stack`, `CapturedOutput`, or the script path.
