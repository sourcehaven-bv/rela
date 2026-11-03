---
id: BUG-8SROSON
type: bug
title: Regex search on an enum property finds nothing
description: A =~ filter on an enum or custom-type property errors and the search shows that as no results.
priority: medium
effort: s
why1: filter.MatchAll returns an error for =~ on an enum property and queryService.matchesPropertyFilters reads any error as no match.
why2: validateOperatorForType limits enum and custom types to = and != although their values are plain strings.
why3: The restriction was written when enum filters only compared against allowed values; the regex operator was added for strings only.
why4: The predicate engine later allowed =~ on any type and nothing compares the two condition surfaces operator by operator.
why5: The search path swallows filter errors so an unsupported operator looks like an empty result and was never reported.
prevention: Operator support for enum now matches the predicate engine and is pinned by filter unit tests. Surfacing swallowed filter errors in search is noted as a follow-up.
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

A regex filter on an enum property finds nothing. In the GRC demo,
`prop:niveau=~kritiek` on the data-entry search returns no rows although
entities with `niveau: kritiek` exist.

`filter.validateOperatorForType` allows only `=` and `!=` on enum, custom
`types:` and legacy status/priority properties, so `filter.MatchAll` returns
`operator "=~" not supported for enum property`.
`queryService.matchesPropertyFilters` (internal/dataentry/queryservice.go)
treats any match error as "no match", so the search answers an empty list
instead of an error.

The predicate engine (`predicatefns.FromFilter`, `when:` conditions) already
applies `=~` to any property type, so the two condition surfaces disagree.

## Reproduction

`TestMatchEnumEdgeCases` and `TestMatchCustomType` in internal/filter: a filter
`status=~dra` on an enum value `draft` errors instead of matching.

## Fix

Allow `=~` on enum and custom-type properties in `validateOperatorForType`, and
match the regex against the stored value in `matchEnum` before the allowed-value
check (a pattern is not a value). Update the operator tables in
docs/metamodel.md and its docs-project mirror.

Out of scope: the search still swallows other filter errors (for example an
unknown enum value) into an empty result. Noted for a separate ticket.
