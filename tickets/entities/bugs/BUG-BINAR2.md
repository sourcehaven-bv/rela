---
id: BUG-BINAR2
type: bug
title: 'Search structured filters and sort: match hidden field values'
description: /_search structured terms such as status:value match values of fields the principal cannot see, so a hidden value can be recovered by search.
priority: high
status: backlog
---

## Description

Found while fixing BUG-WJSYQT. `/_search` structured filters (`prop:value`,
`status:value`) and the `sort:` clause evaluate fields the principal cannot see
under `visible:`. Free-text search already searches only visible text
(BUG-IPO4UH), and list `filter[]`, `sort` and `?q=` are gated by BUG-WJSYQT, but
the structured terms and `sort:` of `/_search` are not.

The same raw evaluation reaches `_position?scope=search`. It resolves the scope
through `executeQuery`, so its prev/next neighbours and its total are computed
from a search filtered and sorted on raw values. A guess at a hidden value can
be confirmed by whether an entity is in scope, and the order of prev/next
discloses the hidden sort key.

## Fix

Apply the same field gate as the list path (`FieldVisibleOnEveryRow`) to every
structured term and every `sort:` key before the searcher runs, answering 400
`property_not_visible`. The gate must sit where `executeQuery` parses the query,
so `/_search` and `_position?scope=search` both pass through it. Pin with corpus
cases for `/_search` and for `_position?scope=search`.
