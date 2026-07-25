---
id: BUG-R9EHKV
type: bug
title: DisplayTitle bypasses the hidden-primary-property fallback on four surfaces (views, mentions, analyze, settings)
description: When an entity type's primary (display) property is hidden by ACL policy, `stripHiddenProperties` (internal/dataentry/affordances.go:905-916) rewrites the wire `Title` to the bare entity ID so the hidden value cannot leak via the display title. Four surfaces call `Meta.DisplayTitle(...)` directly and never reach that fallback, so the hidden property's value is emitted verbatim as a title string. This is a distinct invariant from property-map stripping — a fix scoped to `sections.go` property values will NOT close it.
priority: high
effort: s
why1: Four surfaces emit an entity title built from a property the caller is not permitted to read — internal/dataentry/sections.go:168 (section entity titles and, via groupBy at :288, group headings), internal/dataentry/mentions.go:74 (`v1.Mention.Title` on `ViewResponse.Mentions`), nine sites in internal/dataentry/analyze.go (145, 166, 192, 327, 345, 367, 389, 424, 464), and internal/dataentry/settings_handlers.go:243 (`APIRelationTarget.Title`).
why2: Each calls `s.Meta.DisplayTitle(e.ID, e.Type, e.Properties)` directly against the raw property map. The hidden-primary fallback lives inside `stripHiddenProperties`, which only runs on the `v1.Entity` serializer path (`entityserializer.forWire`). These four surfaces build their own DTOs and never call it.
why3: The fallback was implemented as a side effect of the property-stripping routine rather than as a property of `DisplayTitle` itself. `DisplayTitle` is a pure metamodel helper with no ACL awareness and no ctx parameter, so it cannot enforce the invariant at the point where the title is actually constructed — every caller must remember to post-process.
why4: Title-safety and property-safety are two invariants enforced by one function. Nothing names the title invariant separately, so surfaces that legitimately do not need property stripping (mentions, analyze issue records, relation-target pickers) have no signal that they still owe a title check. internal/dataentry/settings_handlers.go:243 is the worst case — it has neither an entity-level read gate nor a field-level strip, over the ungated `listFromStoreByTypes` lister.
why5: 'Same systemic root cause as BUG-9QL9XV — field visibility is enforced as a per-surface convention rather than a wire-boundary invariant — but with a sharper edge: a title leak is invisible to any test that asserts on the `properties` map, because the value travels in a differently-named field. No test asserts the surface-agnostic property "a hidden value''s bytes appear nowhere in any response", which is the only formulation that covers both variants.'
prevention: Byte-level wire-boundary test using a sentinel value, asserted over every registered route rather than a hand-listed set — see the shared measure proposed on BUG-9QL9XV. A sentinel scan catches title leaks and property leaks with a single assertion precisely because it does not care which field carries the bytes.
status: in-progress
---

## Surfaces

| Site | DTO field | Entity-level gate | Status |
| ---- | --------- | ----------------- | ------ |
| `sections.go:168` | `SectionEntityData.Title` | yes (view scope) | **fixed** — entity redacted before `DisplayTitle` (view reader routing) |
| `sections.go:288` | `GroupData.GroupName` | yes (view scope) | **fixed** — same; grouped value comes from the redacted map |
| `mentions.go:74` | `v1.Mention.Title` | yes | open (handles git-crypt only, not ACL visibility) |
| `analyze.go` ×9 | analyze issue records | yes (TKT-QU7REX) | open |
| `settings_handlers.go:243` | `APIRelationTarget.Title` | **no** | open (worst case: no gate at all) |

## Remaining scope (in-progress)

The `_views` title/group surfaces are closed by the same commit that fixes
BUG-9QL9XV (routing `executeView` through the redacting `viewReader`, whose
`Redact` recomputes `_title`). The three non-view surfaces — `mentions.go`,
`analyze.go`, `settings_handlers.go` — do NOT flow through that reader and still
leak. This bug stays open for them; each needs its own redaction at its builder.
`TestACLViews_RedactsHiddenPrimaryTitle` pins the view surface.

`mentions.go:74` is notable: it already handles git-crypt inaccessibility via
`lockedReasonFor` but not ACL field visibility — the two mechanisms were
conflated (see `internal/entity/entity.go:37`, `Inaccessible` is unrelated to
the `visible:` policy block).

## Relationship to BUG-9QL9XV

Shared root cause, different invariant. BUG-9QL9XV covers hidden property
*values* reaching the wire through `_views` section payloads; this bug covers
hidden property values reaching the wire as *titles*. They are tracked
separately so a fix scoped to `sections.go` property values does not
accidentally close this one on paper.
