---
id: BUG-57A53F
type: bug
title: Free-text search judges hidden fields one hit at a time
description: 'Under a field-hiding policy, free-text search computes a field verdict per hit with no batch step, so a traversing when: costs one store query per hit.'
priority: medium
effort: m
why1: 'search.Visible.fieldVisible judged one hit at a time: for each hit it loaded the entity and called the consumer''s HiddenFieldsFunc, which evaluated the grant''s when: for that one row.'
why2: HiddenFieldsFunc was a per-row callback from the search layer into the ACL with no batch step. It was the only way the searcher could learn which fields a row hides.
why3: Field visibility was decided in Go per row instead of being compiled into the store query, so the searcher could not answer it for a page at once.
why4: 'A visible: when: could be any predicate, including ones the store cannot answer (not, has_role), so a per-row callback was the only general way to evaluate it.'
why5: Nothing required an ACL condition used on a collection path to lower to a store query, and the collection-reads budget rule had no test on the free-text path until BUG-WJSYQT added one that pinned the per-hit cost instead of forbidding it.
prevention: 'Every visible: when: must lower through queryplan.LowerScope or the policy fails to load, so the store answers field visibility per page and no per-row ACL callback exists. TestQueryBudget_ListFreeTextUnderFieldHiding pins zero entity loads and a constant traversal count at 10 and 50 hits, and TestListBodies_QueryShapesKeepTheBound pins zero bodies on ?q=. CLAUDE.md records the search rule.'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

## Description

Found while addressing the review of BUG-WJSYQT. Under a policy that hides
fields with `visible:`, a free-text search drops a hit whose text matched only
hidden fields. To decide that, `search.FieldVisibleSearcher.SearchVisibleFields`
calls the consumer's `HiddenFieldsFunc` once per hit. That callback has no batch
hook, so a `visible:` grant whose `when:` uses `related()` runs one traversal
query per hit instead of one per page. The in-memory implementation
(`search.Visible`) also loads each hit's entity separately to tell which fields
matched; pgstore already reads headers per page and bodies in one batch.

Every free-text path pays this: list `?q=` (since BUG-WJSYQT), `/_search`, and
the MCP searcher (`visibility.Searcher`). With up to 1000 hits per request, the
cost grows with the number of hits, which `.claude/rules/collection-reads.md`
forbids.

`TestQueryBudget_ListFreeTextUnderFieldHiding` in
`internal/dataentry/acl_when_related_budget_test.go` pins the current per-hit
terms at exactly one entity load and one traversal per hit.

## Fix

Give the hidden-field callback a batch step, in the style of
`visibility.TraversalPrimer`: the searcher hands each page of admitted rows to
the consumer once, and the consumer primes its field verdicts before judging the
rows. In `search.Visible`, judge a page from headers first and load bodies only
for the rows whose verdict depends on them, as pgstore does. Then turn the
budget test into a size-independence assertion.
