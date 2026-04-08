---
id: BUG-D2QS
type: bug
title: Data-entry SPA Loading spinner hangs after mutation-heavy navigation sequences
description: 'When the fast-check fuzzer runs in --mutate mode (create/edit actions actually submit forms) against a Firefox BrowserContext, after ~5-100 iterations a `page.goto(/v2/list/all_tickets)` shows the SPA stuck on the ''Loading...'' spinner indefinitely. The post-mortem HTML capture shows the Vue app mounted but `loading === true` — App.vue is stuck awaiting schemaStore.load() which is waiting on GET /api/v1/_schema. Server logs show no errors and no request rejection, which means the fetch is either timing out on the client side (but axios has no default timeout) or never receiving a response. This is likely related to BUG-DW6H (SPA refetches on every SSE event) and the backlog of file watcher events from repeated mutations overwhelming the server''s response path. Reproducible with: npm run stress -- --mode=fuzz --browser=firefox --num-runs=200 --max-actions=15 --mutate.'
priority: medium
effort: m
why1: SPA's App.vue is stuck awaiting schemaStore.load() — the Loading spinner never clears.
why2: schemaStore.load() is either waiting on a schema/config fetch that never completes, or the response doesn't trigger the watch that clears loading.value.
status: ready
---
