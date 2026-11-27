---
id: BUG-EXJ54CD
type: bug
title: Deep-linked list filter on a hyphenated relation name is dropped
description: A list deep link such as /list/all_tickets?filter[belongs-to]=devops lost its filter on a direct load, reload or shared link, because the SPA URL parser accepted only identifier-style names.
priority: medium
effort: s
why1: parseFilterQueryParams drops any filter key whose name fails PROPERTY_NAME_RE (/^[a-zA-Z_][a-zA-Z0-9_]*$/), so belongs-to is dropped.
why2: The allowlist was written as an identifier rule to block prototype pollution, not derived from the metamodel's name rule, which allows dashes, dots, spaces and non-ASCII letters.
why3: The unit test asserted that a-b is rejected, and the e2e relation filter test uses implements, which has no hyphen, so no test loaded a deep link with a real hyphenated name.
why4: The identifier regex also accepted __proto__, so the guard it was justified by did not hold; toEqual ignores prototypes, so the old test passed anyway.
why5: Client-side name checks are written by hand instead of mirroring the server's rule, and tests use synthetic names rather than names from a shipped schema.
prevention: Vitest cases pin that every name ValidateSchemaName accepts survives a URL round trip and that __proto__, constructor and prototype are dropped with the result's prototype unchanged (AM-filter-url-accepts-schema-names).
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

## Description

A direct load of a list URL with a filter on a hyphenated name, such as
`/list/all_tickets?filter[belongs-to]=devops`, sends no filter to the API.
`filter[status]=open` works. Picking the value in the filter control works,
because that path does not re-parse the URL. A reload or a shared link loses the
filter.

## Cause

`parseFilterQueryParams` in `frontend/src/utils/filters.ts` drops every key
whose name fails `PROPERTY_NAME_RE` (`/^[a-zA-Z_][a-zA-Z0-9_]*$/`). Relation and
property names in the metamodel may contain dashes, spaces and dots
(`metamodel.ValidateSchemaName` is a blocklist), so valid names are dropped.

## Fix

Accept every name `ValidateSchemaName` accepts, and keep rejecting `__proto__`,
`constructor` and `prototype`.
