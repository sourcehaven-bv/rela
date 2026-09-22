---
id: TKT-58ID95
type: ticket
title: Reconcile size gates and streaming across the three download endpoints
kind: enhancement
priority: low
effort: s
status: backlog
---

## Problem

Three endpoints serve bytes as a download, and they do not agree:

| Endpoint | Streaming | Size gate | Cache-Control |
| --- | --- | --- | --- |
| `/api/v1/.../_attachments/...` | `io.Copy` | yes | none (stored content) |
| view export | buffered | via transform output cap | `no-store` |
| `/api/command-file/{token}` | `io.Copy` | **none** | `no-store` |

The command-file route streams whatever the script wrote, with no limit, no
`Range` support and no conditional requests.

Raised as RR-T7Z6UW during the TKT-93FUCV code review.

## Why it was deferred

The producer is an operator-authored script writing inside the project root, so
it is already trusted and can already fill the disk directly — this route does
not widen that. Bundling a streaming rewrite into a security-sensitive ACL
change would have mixed two independent risks in one diff.

## Approach

`internal/dataentry/CLAUDE.md` already records the pattern, from the `custom/`
handler: `http.ServeContent` gives conditional requests, `Range` and correct
`HEAD` in one call. Two constraints it also records, both load-bearing:

- **Keep the explicit `Content-Type`.** ServeContent sniffs when it cannot
infer one, which is the exact behaviour `nosniff` exists to prevent.
- **Keep a pre-read size gate.** ServeContent will otherwise stream a file of
any size.

The point of doing all three together is to decide the limit *once* rather than
pick a different number per endpoint.

## Acceptance criteria

1. The command-file route has a documented size limit, and a test that a file
over it is refused rather than streamed.
2. The three endpoints' limits are either the same constant or differ with a
stated reason.
3. `Content-Type` stays explicit on every path; a test pins that `nosniff`
is not undermined.
