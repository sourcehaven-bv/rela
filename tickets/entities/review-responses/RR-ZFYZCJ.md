---
id: RR-ZFYZCJ
type: review-response
title: The start-offset type boost fails on a mid-word type match
finding: 'The plan''s type-match boost keys on `info.start === 0`, on the assumption that a match beginning at offset 0 of the `"<type> <title> <id>"` haystack is a type-name match. Measured against @leeoniya/ufuzzy 1.0.19: querying `report` against the haystack `FancyReport some word in title` yields `start=5`, not 0 — the match begins mid-type-name because uFuzzy''s intraSplit finds the camelCase boundary. So a user typing `report-some` (a perfectly reasonable way to reach FancyReport) gets `start=5` and no boost, while the boost fires for any query that happens to begin matching at character 0 regardless of whether it matched the type at all. The boost is both over- and under-inclusive. It should test whether the match RANGE overlaps the type segment (whose length is known, since the haystack is composed locally), not whether it starts at zero.'
severity: significant
resolution: Designed out rather than fixed. The entity type is now a FILTER (via the type picker and the existing ?type= parameter), not a scoring dimension, so it is omitted from the haystack entirely and no type boost of any kind is needed. The measured defect (query `report` against `FancyReport` giving start=5, not 0) no longer has a code path.
status: addressed
---

## Evidence

Measured against the installed package (`@leeoniya/ufuzzy` 1.0.19) with haystack
`['FancyReport some word in title']`:

```
"report"      -> start=5  terms=1  needed=1
"report-some" -> start=5  terms=2  needed=2
```

Both are legitimate ways to reach a `FancyReport` entity, and neither would be
boosted by an `info.start === 0` test.

## Required plan change

Because the haystack is composed client-side, the type segment's boundaries are
known exactly:

```ts
const typeLen = e.type.length
// boost when the match overlaps [0, typeLen)
const hitsType = info.start[o] < typeLen
```

Better still, use `info.ranges[o]` (uFuzzy returns match ranges for
highlighting) and test whether any range intersects `[0, typeLen)`. That is
precise, and it is the same data the menu would need for highlighting anyway.

Add a unit test for the mid-word case specifically: query `report-some` must
rank the `FancyReport` entity above an entity that merely contains "report" in
its title. The plan's current AC 2 only covers the leading-match case and would
pass while this bug is live.
