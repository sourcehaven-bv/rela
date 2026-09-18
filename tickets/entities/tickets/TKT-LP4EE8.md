---
id: TKT-LP4EE8
type: ticket
title: Document command renderers fork an unbounded bubblewrap sandbox per request
kind: enhancement
priority: medium
effort: s
status: backlog
---

Found while raising E2E Playwright workers from 2 to 4 (TKT-FP04ZE).

`internal/dataentry/document.go:720` builds its cmdexec runner with no
concurrency bound:

```go
runner, err := cmdexec.New(timeout, maxCommandOutputBytes)
```

`internal/transform/engine.go:39` does the opposite, and the comment there
explains why:

```go
cmdexec.WithMaxConcurrent(defaultMaxConcurrent)  // 4
```

So every concurrent document render forks its own bubblewrap sandbox with no
aggregate cap. The per-command rlimits bound one process; nothing bounds the
total.

## Evidence

Raising E2E workers 2 → 4 put roughly 8 busy processes (a Chromium and a
rela-server per worker) on a 4-core runner. All 12 tests in
`document-edit-button.spec.ts` failed, with the page showing:

> No document content available — Document "feature_summary" may not be
> configured or the entity "FEAT-001" may not exist.

That document renders via `command: ["cat", "{in}"]`. The page itself rendered
promptly, so this is not a timeout on the request: the sandboxed renderer
failed, and cmdexec correctly failed closed. Both Playwright retries were
burned, so this was not masked as a slow green.

It is intermittent — one run at 4 workers passed, the next failed — which is
consistent with contention rather than a deterministic break.

## Why it matters beyond CI

The E2E runner is just where it surfaced. A production rela-server rendering
`command:` documents for several concurrent users has the same shape, and
`internal/transform` already treats that as worth bounding. The failure is
user-visible (a document silently renders as "not available") and load
dependent, so it would be hard to reproduce from a bug report.

## Suggested fix

Give the document runner the same `WithMaxConcurrent` treatment as the transform
engine, and build it ONCE rather than per request — `executeCommand` currently
constructs a fresh runner on every call, so even a bounded pool would bound
nothing if it stayed per-request. That mirrors the existing rule in CLAUDE.md
for the transform engine: "The transform engine must be built ONCE and shared,
not per request — it owns the bounded pool, so a per-request engine gives every
request its own pool and the concurrency cap bounds nothing."

Raising the E2E worker count is gated on this.
