---
id: TKT-79WJ6G
type: ticket
title: 'seqtrace: sequence diagrams of traced request flows'
kind: enhancement
priority: low
effort: l
status: done
---

## Description

Add `tools/seqtrace`, a diagnostic tool that shows how rela handles a request as
numbered Mermaid sequence diagrams, one participant per package.

- `seqtrace overlay` copies the Go sources of `internal/` and `cmd/`, injects
calls to a small runtime, and writes an `overlay.json` for `go build -overlay`.
The working tree does not change and line numbers are kept.
- The runtime records each call with a short summary of its arguments and
results. It links calls across goroutines through `ctx`, `go` statements and
closures. `SEQTRACE_ROOT` limits recording to HTTP requests.
- `seqtrace diagram` draws one diagram per request, an index page, and one
page per diagram.
- `just seqtrace-demo` copies the checkout, builds `rela-server` on the
postgres build, seeds the perf project with its `acl.yaml`, sends ten requests
as three users, and opens the diagrams.

Normal builds never link the runtime.
