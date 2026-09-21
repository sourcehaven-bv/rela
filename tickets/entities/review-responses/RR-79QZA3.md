---
id: RR-79QZA3
type: review-response
title: Postgres search text excludes the entity type, making the headline feature structurally impossible there
finding: 'RR-DLL40G covers LinearSearch; postgres has the same whole-string substring behaviour AND a second, deeper problem. pgstore/search.go:59 does `needle := strings.ToLower(text)` and buildSearchSQL emits a single `search_text LIKE ''%'' || $1 || ''%''` with that whole needle, so a space-joined multi-term query matches strictly less, exactly as on LinearSearch. Worse: entitySearchText (pgstore/entity.go:1304-1330) composes the indexed text as id + properties + content joined by NEWLINES, and does NOT include the entity TYPE at all. So on postgres the ticket''s headline feature — matching the token `fancy` against the entity type `FancyReport` — cannot work regardless of the client-side scorer, because no candidate is ever returned on the strength of its type. The newline separators additionally prevent a space-joined needle from matching across the id/title boundary. This was read from the code, not executed (no RELA_TEST_DATABASE_URL available), but the code is unambiguous.'
severity: significant
resolution: Superseded by RR-77AMZO. With a visible type picker the type is filtered via the existing ?type= parameter (an indexed equality), never matched as free text, so pgstore's search_text needs no change and no backfill. The underlying observation stays true and is recorded in RR-B0L1L0 for any future free-text type matching.
reason: 'Superseded rather than disputed. The observation is correct — pgstore''s entitySearchText composes id + properties + content and omits the entity type, so a type name can never match as free text on postgres. It stops being a defect in THIS ticket because the operator chose a type-picker UI (RR-77AMZO): the type now reaches the server as the existing ?type= parameter, which compiles to an indexed equality against entities_type_idx, rather than as text matched against search_text. Fixing it here would require a full-table backfill inside pgstore.Migrate''s single transaction under an advisory lock for no behavioural gain in this feature. Carried forward under RR-B0L1L0 as a candidate standalone backend ticket.'
status: wont-fix
---

## Evidence

- `internal/store/pgstore/search.go:59` — `needle := strings.ToLower(text)`,
the whole query text.
- `buildSearchSQL` (`search.go:161-163`) — one
`search_text LIKE '%' || $1 || '%'` predicate with that needle.
- `internal/store/pgstore/entity.go:1304-1330` — `entitySearchText` composes
`id \n <properties> \n <content>`. The entity **type** is absent.

## Consequence

Three backends, three different behaviours for the same query:

| | recall model | type searchable? |
|---|---|---|
| bleve | per-word OR, fuzziness 1 | yes (`type` field indexed) |
| LinearSearch | whole-string substring | no |
| pgstore | whole-string substring (SQL LIKE) | **no** |

The ticket's motivating example needs the type to be matchable. It is on exactly
one backend.

## Required plan change

The plan's scope says "reworking backend relevance scoring" is out of scope.
That boundary makes the stated goal unreachable on two of three backends, so
either:

1. **Move the boundary** — add the type to `entitySearchText` (a small, contained
pgstore change plus a migration/reindex) and accept LinearSearch as the
test-only backend it is documented to be; or
2. **Shrink the acceptance criteria** — state plainly that cross-field type
matching is a bleve-build feature, and that pg/memory degrade to title-and-id
matching.

Option 1 is preferable and is probably small. Either way the plan must say
which, because a reviewer or QA pass will otherwise find the feature "broken" on
postgres and be unable to tell whether that was intended.

Verify against a live database (`RELA_TEST_DATABASE_URL`, `just test-postgres`)
before implementation — this finding is code-read, not executed.
