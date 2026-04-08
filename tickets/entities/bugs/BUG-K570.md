---
id: BUG-K570
type: bug
title: Stress harness wedges after ~8 minutes with 4 concurrent Firefox users
description: 'When the stress harness runs more than ~8 minutes with 4 concurrent Firefox BrowserContexts driving the watcher-pressure scenario, the runner''s Node event loop is throttled by 60-150x: schema canary rate drops from 5/s to 0.03-0.08/s, the file-touch ops stop firing, the progress timer stops emitting checkpoints, and Playwright click ops hang past their 10s nominal timeout. After ~15 minutes the system briefly recovers (canary rate returns to ~4.6/s) before wedging again. Server-side is innocent: the rela-server log shows normal file-change processing throughout, the schema canary''s actual fetch latency stays sub-50ms when fetches do happen, and a goroutine dump captured during the wedge shows ~25 goroutines, no contention. The wedge is in the runner / Playwright / Firefox stack. Most likely cause: Playwright''s CDP/BiDi protocol channel to multiple Firefox sessions becomes saturated with browser-event traffic (1000+ console errors and thousands of navigation events streaming over a single shared protocol pipe), starving Node''s main event loop. Workarounds to try: launch one Firefox process per user (browserType.launch per loop) instead of sharing one Firefox; reduce default Firefox user count to 2; disable console-error forwarding (we have to keep it for invariants but could batch it).'
priority: medium
effort: s
why1: Long Firefox stress runs hang the harness mid-soak, masking real server-side metrics behind runner-side noise.
status: ready
---
