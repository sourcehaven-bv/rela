---
id: TKT-ZIEV
type: ticket
title: Enable HTTP/2 (h2c) on rela-server for tail-latency improvements
kind: enhancement
priority: low
effort: xs
status: ready
---

## Update — original hypothesis disproved by experiment

The original framing of this ticket claimed that HTTP/1.1 per-host
connection-pool exhaustion was the cause of BUG-FMS1 ("Firefox hangs on page
load"). **The experiment refuted that.** A 7-minute Firefox stress run with the
h2c fix in place produced **the same rate of "Firefox can't establish a
connection to /api/v1/_events" errors** as the pre-fix 30-minute run, normalised
per-minute.

| Window | Pre-fix (HTTP/1.1) | Post-fix (h2c) |
|---|---|---|
| First ~7.5 min, "can't establish SSE" errors | 51 | 53 |

If pool exhaustion was the cause, h2c should have reduced these to zero. It
didn't. The user's "long long time" Firefox hang has some other root cause that
we have not yet isolated.

## What h2c IS still good for

The same experiment showed real, repeatable server-side latency wins:

| Metric | HTTP/1.1 | h2c | Change |
|---|---|---|---|
| Schema canary p99 | ~10 ms | **5.3 ms** | **1.9× better** |
| Schema canary max | ~40 ms (sometimes 465 ms) | **13.3 ms** | **3-35× better** |
| Server 5xx | 0 | 0 | same |
| SSE over h2 | n/a | works (verified with curl) | works |
| HTTP/1.1 fallback | works | **still works** (verified) | preserved |
| Security middleware | applies | still applies | preserved |

The latency improvement is consistent with what HTTP/2 multiplexing should give:
fewer head-of-line blocking events, no per-host queuing, no slow-start ramp on
every new connection. It is **worth keeping** as a small performance
improvement, just not under the original framing of "fixes the user-reported
hang".

## Revised priority

Demoted from `high` to `low`. This is now an opportunistic perf improvement, not
a bug fix. Pick it up when there is appetite for a small low-risk net.

## What still needs investigation

The user-reported Firefox hang is NOT explained by:
- Server-side lock contention (disproved by goroutine dumps — see BUG-FMS1)
- HTTP/1.1 connection-pool exhaustion (disproved by this experiment)

Plausible remaining hypotheses:
1. The SPA's SSE reconnection lifecycle has a Firefox-specific race
condition. Every SSE failure in the harness correlates with a SPA navigation
event, suggesting the close/reopen sequence is racy.
2. The user's Firefox profile has specific extensions (uBlock, NoScript,
Privacy Badger) or settings (ETP, HTTPS-Only mode) that are intercepting or
blocking subresources.
3. Something specific to the user's actual project at `~/pim` that none
of the harness scenarios reproduce.

The HTTP/2 fix is orthogonal to all of these.

## Original ticket content (preserved for context)

The text below was the initial framing before the experiment. Kept for audit
trail; superseded by the "Update" section above.

---

## Problem

The user reports that the rela-server data-entry web app intermittently hangs in
**Firefox** for "a long long time" before any subresources load. HARs show all
subresource and `/api/v1/*` requests in `status: 0` with empty timings (i.e.
requests created in Firefox but never sent over the wire), then later either
succeeding in a single fast burst or remaining failed. See `BUG-FMS1` for the
original observation and the dead-end lock-contention hypothesis.

## Approach

Wrap the existing `http.Server` handler with `h2c.NewHandler` from
`golang.org/x/net/http2/h2c`, which is **already a transitive dependency**
(`go.mod` shows `golang.org/x/net v0.47.0`), so no new dependencies.

```go
import (
    "golang.org/x/net/http2"
    "golang.org/x/net/http2/h2c"
)

h2s := &http2.Server{}
srv := &http.Server{
    Addr:              addr,
    Handler:           h2c.NewHandler(handler, h2s),
    ReadHeaderTimeout: 10 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      0,
    IdleTimeout:       120 * time.Second,
}
```

## Acceptance criteria (revised)

1. `cmd/rela-server/main.go` wraps the handler with `h2c.NewHandler`. ✅ done
2. ~~The 30-minute Firefox soak shows zero "can't establish a connection" errors.~~ **REVISED**: error count unchanged. Document instead that p99 schema canary latency improves by ~2× and max by 3-35×.
3. The Chromium baseline run still passes with the same metrics. ✅ verified
4. `curl http://127.0.0.1:PORT/api/v1/_schema` still returns 200 (HTTP/1.1 compatibility). ✅ verified
5. The pprof endpoint still functions. ✅ verified
6. SSE (`/api/v1/_events`) still streams events over both HTTP/1.1 and HTTP/2. ✅ verified with curl
7. Security middlewares still apply. ✅ verified
