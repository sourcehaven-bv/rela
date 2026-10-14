---
id: BUG-57A53F
type: bug
title: Free-text search judges hidden fields one hit at a time
description: 'Under a field-hiding policy, free-text search computes a field verdict per hit with no batch step, so a traversing when: costs one store query per hit.'
priority: medium
status: backlog
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
