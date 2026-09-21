---
id: TKT-NQWTCG
type: ticket
title: Entity type is not free-text searchable in /_search on any backend
kind: enhancement
priority: low
effort: m
status: backlog
---

## Problem

An entity's **type** cannot be matched as free text on any search backend. A
user searching `fancyreport` gets nothing back for an entity of type
`FancyReport`, even though the type is displayed everywhere in the UI.

Established by measurement during TKT-6MZ42J (see RR-B0L1L0):

- **bleve** indexes a `Type` field but never searches it. `boostedFields` covers
`primary`, `properties`, `content` and `all`; `entityToDoc`'s `all` composite is
`{ID, primary, props, Content}` — the type is absent from both. Measured: an
entity of type `FancyReport` returns `[]` for `fancyreport`.
- **LinearSearch** matches via `MatchTextFields`, which does not consider the type.
- **pgstore** does not include the type in its indexed search text either.

Note **sqlite uses bleve**, not FTS5 (`appbuild_sqlite.go`), so it inherits the
bleve behaviour rather than needing separate work.

## Why it was left out of TKT-6MZ42J

That ticket wanted `fancy-some-word-in-title` to find a `FancyReport`. It
shipped a visible type **picker** instead, which is better UX for the mention
menu (the user disambiguates rather than the system guessing) and needed no
backend change. But the underlying gap is real and affects `/_search` generally,
not just the mention menu — the search page has the same blind spot.

## Cost, measured rather than estimated (RR-V9R8S9)

- A full-table backfill under an advisory lock to re-index existing rows.
- A five-site change, including `MatchTextFields`, which is the **conformance
ground truth** other backends are checked against (`RunVisibleSearchTests`). All
backends must move together or the suite fails.
- A false-positive blowup: `@pro` would match every `project` *and* every `policy`
row on type alone, which is precisely the noise the picker avoids.

## Suggested approach

Do **not** fold the type into the existing free-text field. A separate,
explicitly weighted type dimension (or a `type:` query prefix — `searchparser`
already parses `type:`) keeps the recall opt-in and avoids the blowup. Whatever
is chosen must:

- Update `MatchTextFields` and every backend together, with the conformance suite
as the gate.
- Stay ACL-safe: recall still flows through `search.VisibleSearcher`, and the type
is not confidential (root CLAUDE.md: config is not a secret), so no new gating
is needed — but no new path may widen what a principal can see.

## Out of scope

The mention menu, which is solved by the picker.
