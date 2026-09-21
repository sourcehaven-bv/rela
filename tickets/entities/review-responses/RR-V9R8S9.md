---
id: RR-V9R8S9
type: review-response
title: Putting the type into search_text costs a full-table backfill, a 5-site conformance change, and a false-positive blowup
finding: 'RR-B0L1L0 establishes the type must become matchable. The obvious implementation — append it to the searchable text blob — carries four costs the plan must account for. (1) FIVE code sites, not two: search.MatchText and search.MatchTextFields (internal/search/filter.go:103,129) are the declared conformance GROUND TRUTH every backend is pinned against (storetest/visiblesearch.go:325, bleveindex/fieldmatch.go:45), and pgstore/entity.go:1302 states entitySearchText ''mirrors search.MatchText''s field selection exactly''. Changing only pg + bleve fails the conformance suite. (2) A MANDATORY full-table backfill inside pgstore.Migrate''s single transaction under pg_advisory_xact_lock — migration 0014 is the template and its own comment warns of a pause ''proportional to the table — seconds at 20k rows''; at 100k+ that is a minutes-long lock. TestMigration0014_SearchTextRebuildMatchesStore pins SQL/Go parity and must be extended or the two silently drift. (3) FALSE-POSITIVE BLOWUP, the real perf hazard: type names are short common words (task, person, policy, project), so typing ''@pro'' would match every project AND every policy row — at perfseed scale 1 that is 300 + 1500 extra rows in one candidate set. The trigram recheck and the similarity() sort both run over the FULL match set before LIMIT, and the SELECT pulls every matched row''s full content across the wire. MIN_SEARCH_LEN=2 puts users in this regime by default. (4) Ranking perturbation: type lands inside the first 1024 bytes that similarity(left(search_text,1024),...) scores, so existing query relevance shifts.'
severity: significant
resolution: 'Superseded by RR-77AMZO. The type-in-blob route is not being taken, so the backfill, the five-site conformance change, the ranking perturbation and the false-positive blowup all fall away. Two recommendations from this finding SURVIVE and should be carried into the plan regardless: (1) there is no query-count budget test covering the FREE-TEXT search branch that the @ menu actually uses — TestQueryBudget_SearchIsSizeIndependent only exercises the structured ?q=type:ticket branch; (2) pg_trgm needs 3 characters, so the menu''s MIN_SEARCH_LEN=2 means the first search a user triggers cannot use the trigram index and degrades to a full scan pulling full row bodies. Both are pre-existing and worth their own follow-up ticket.'
reason: 'The route this finding costs out is not being taken, so its costs are avoided rather than accepted. With the type-picker design (RR-77AMZO) the entity type is filtered via ?type=, never written into search_text, so there is no migration, no full-table backfill, no five-site conformance change across MatchText/MatchTextFields/entitySearchText/entityToDoc, no ranking perturbation of existing queries, and no false-positive blowup from short prefixes of common type names. Two recommendations from this finding are NOT dropped: they are carried into the plan''s Out of Scope section as follow-up candidates — (1) no query-count budget test covers the free-text search branch the @ menu actually uses, since TestQueryBudget_SearchIsSizeIndependent only exercises the structured branch; (2) pg_trgm requires 3 characters while MIN_SEARCH_LEN is 2, so the first search a user triggers cannot use the trigram index. Both are pre-existing and independent of this ticket.'
status: wont-fix
---

## The lower-risk alternative, already available in the code

`e.type = ANY($n)` is **already** in the generated search SQL
(`pgstore/visiblesearch.go:284-286`), and `e.type` is indexed twice
(`entities_type_idx`, `0001_init.sql:46`; `entities_type_id_idx`,
`0014_read_indexes.sql:32`).

Resolving the needle against the metamodel's type names **in Go** — a cheap
in-memory lookup the handler already holds as `svc.Meta.Entities`
(`queryservice.go:135`) — and matching on type as an indexed equality avoids:

- the migration and the full-table backfill,
- the `MatchText` / `MatchTextFields` ground-truth change and the conformance
churn,
- the ranking perturbation,
- the false-positive blowup (an indexed equality on a resolved type name is
exact; a substring in a text blob is not).

**One important caveat, verified.** The existing `e.type = ANY(...)` is
**AND**ed into the query as a *filter* — it narrows the result set. Using it to
*broaden* ("rows matching this text **OR** rows of this type") means changing
that predicate's composition, not just reusing it. That is still far smaller
than the blob change, but it is not free and must not be described as reuse.

## Cost of the type-in-blob route, for the record

| dimension | cost |
|---|---|
| code sites | `MatchText`, `MatchTextFields`, `entitySearchText`, `entityToDoc`, new pg migration |
| migration | full-table `UPDATE` in one transaction under an advisory lock; minutes at 100k rows |
| index size | negligible (trigram posting lists lengthen; dictionary barely grows) |
| write path | negligible (one more `WriteString` on an existing allocation) |
| read path | **materially worse** on short prefixes of common type names |
| ranking | existing queries shift |

## ACL dimension, do not miss this

`fieldVisibleForEntity` (`pgstore/visiblesearch.go:191`) drops a hit when every
field it matched is hidden. A type match must map to a field name that is
**never** in the hidden set, alongside `FieldID` and `FieldContent` which
`internal/search/visible.go:104-105` documents as "never property-gated".
Otherwise type-matched hits are silently dropped under field redaction. The type
is not confidential (root CLAUDE.md: "The configuration is not a secret"), so
this is the correct treatment — but it has to be explicit.

## Required plan change

Choose between the two routes and record the reasoning. Then add, either way:

- **A free-text query-count budget test.** `TestQueryBudget_SearchIsSizeIndependent`
(`querybudget_test.go:219`, `searchBudget = 3`) uses `?q=type:ticket`, a
*structured* query that takes the `visibleListByTypes` branch. **The free-text
branch the `@` menu actually uses is unpinned.**
- **An EXPLAIN test** proving the search shape still uses the trigram index, per
the root CLAUDE.md rule. No existing EXPLAIN test covers the search `LIKE`
shape.
