---
id: RR-1JZTB0
type: review-response
title: Empty interior path segment let a standalone document render at the anchored export URL shape
finding: 'handleV1Documents split the path with strings.Split and trimmed only ONE trailing empty segment, so /_documents/sales//_export produced parts ["sales", "", "_export"], matched the 3-segment export case, and called handleV1ExportDocument with an empty entity id — which dispatches on entityID == "" to the STANDALONE resolver. Verified by execution: the request returned 200 and rendered the standalone document at the anchored URL shape. That is exactly what internal/dataentry/CLAUDE.md forbids: ''Never let one URL shape serve the other kind... Do not helpfully fall back to rendering with an empty or guessed entry id.'' net/http''s ServeMux 307-redirects the // spelling before a handler sees it, so the route was unreachable through the normal server, but that is a property of the mux rather than of the router: the package''s own tests call handleV1Documents directly, and a percent-encoded %2F survives ServeMux cleaning entirely. Found independently by both the code reviewer and the security reviewer.'
severity: significant
resolution: Replaced the single trailing-segment trim with a slices.Contains(parts, "") rejection covering every position, so an empty segment is a 400 before any dispatch. The comment states why this is not defensive tidying and why ServeMux's normalization must not be relied on. Pinned by TestExportDocument_RouteShapes, which now walks 16 path shapes including both the // and %2F spellings and asserts the renderer never ran on each rejection.
status: addressed
---

## Verification

Confirmed by execution before fixing, against the committed code:

```
/api/v1/_documents/sales//_export   -> 200 renders=1 body=# doc sales entry=[]
/api/v1/_documents/report//_export  -> 400 renders=0 (kind mismatch)
```

The first line is the defect: a standalone document served at the anchored
shape. After the fix both return 400 with `renders=0`.

## Why it was worth fixing despite being unreachable in production

Three reasons, in order of weight:

1. The guarantee belonged to `http.ServeMux`, not to this router. Any future
direct mount, a reverse proxy that does not normalize, or a test calling the
handler directly loses it — and the package's own tests already do exactly that.
2. `%2F` survives `ServeMux` cleaning, so the normalization was never total.
3. `entityID == ""` was doing double duty as "standalone" and as "the router
produced an empty segment". A sentinel that the router can manufacture by
accident is the actual defect; rejecting empty segments removes the only way to
manufacture it.

The deeper fix the code reviewer suggested — making the standalone case
unrepresentable with a `kind` field instead of a sentinel empty string — is
recorded as a follow-up rather than done here, since it touches
`RenderDocumentMarkdown` and `exportBaseName` too.
