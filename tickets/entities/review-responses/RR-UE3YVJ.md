---
id: RR-UE3YVJ
type: review-response
title: Splitting removes the single-token invariant that keeps filter syntax from being injected mid-query
finding: 'parseMentionQuery does not terminate on '':'', so queries like @status:open already reach /_search. That is pre-existing and mostly benign — ParseQuery only strips four known prefixes (type:, prop:, status:, sort:) via CutPrefix, so an arbitrary `foo:bar` falls through to free text. What the plan changes is the invariant that made it safe: mentionQuery.ts guarantees the query is a SINGLE whitespace-free token, so at most one filter prefix can ever match, at the start. After splitting, a multi-segment query can inject a filter into the MIDDLE of the sent string — `@my-sort:x-thing` becomes `my sort:x thing`, and `sort:x` is then parsed as a sort clause rather than as the literal text the user typed. Measured through the real ParseQuery: ''sort:x'' yields SortClauses=[{x asc}] with IsEmpty()=TRUE (so executeQuery returns nil, i.e. no results at all), ''status:open'' becomes a property filter, and ''prop:a'' produces a parse error and IsEmpty()=TRUE. tokenize() additionally treats ''"'' as a quote toggle and ''\'' as an escape, so an unbalanced quote in one segment re-tokenizes everything after it.'
severity: significant
resolution: Cannot arise under the revised design. The query is sent to /_search unmodified, so mentionQuery.ts's single-whitespace-free-token invariant is preserved and no segment can inject a filter prefix into the middle of the sent string. The selected type reaches the server through the separate ?type= parameter, chosen from schemaStore.entityTypes — an allowlist by construction, so a free-typed string never becomes a type value.
status: addressed
---

## Evidence

Through the real `searchparser.ParseQuery`:

```
"foo:bar"      -> words=[foo:bar]                      (safe, free text)
"sort:x"       -> sorts=[{x asc}]   IsEmpty=TRUE       -> nil results
"status:open"  -> props=1                              -> becomes a filter
"prop:a"       -> errs=[invalid property filter…] IsEmpty=TRUE
"a\b"          -> words=[ab]                           (backslash eaten)
```

`parser.go:42-118` strips only the four known prefixes; `tokenize`
(`parser.go:139-185`) handles quotes and escapes.

## Why the split changes the risk

Today the query is one whitespace-free token, so a filter prefix can only ever
appear at position 0 and the blast radius is one obvious case. After splitting,
any segment can become a filter keyword. `@my-sort:x-thing` is an ordinary thing
to type and would silently return **no results at all** (IsEmpty short- circuits
`executeQuery` to nil), which reads to the user as "this entity does not exist".

## Required plan change

If the split ships, sanitize each segment before joining:

- Drop or escape a segment that begins with `type:`, `prop:`, `status:` or
`sort:`.
- Strip `"` and `\` from segments, or quote the whole thing.

State the rule as an allowlist (segments are `[A-Za-z0-9']+` after the split
anyway, which makes this nearly free) rather than a blocklist of the four
prefixes, so a fifth prefix added to `ParseQuery` later cannot silently
reintroduce the hole.

Add a test for `@my-sort:x-thing` and for a segment containing a quote.
