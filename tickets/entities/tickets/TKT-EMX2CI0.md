---
id: TKT-EMX2CI0
type: ticket
title: Return list group_by groups and date buckets from the server
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

Lists with `group_by:` are split into sections in the SPA
(`useListGrouping.ts`), including empty enum groups and relative date buckets
(`utils/dateBuckets.ts`). Return the groups in the list response as `{key,
label, style, count}` with row ranges. Date buckets use a `tz` parameter from
the reader, by calendar day as today. Add a budget test for the list path.

Part of FEAT-QRI3CFR (step 1.3).
