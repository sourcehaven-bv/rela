---
id: BUG-CZFPMSX
type: bug
title: 'tenanthost: a request racing the end of a lazy provisioning run starts a second run'
description: TestProvision_LazySingleFlight fails about once in 200 runs with 'provisioner calls = 2, want 1'. A lazy request calls Acquire, sees the tenant absent, and then calls provisioning.start. If the org's run succeeds and leaves p.runs between those two calls, start finds no run and no hold, and starts a second Provision. The second create is idempotent in production, but it breaks the single-flight promise and spends a start-rate token and a running slot. The log line about another process in the failing CI output comes from a parallel test that stubs ErrTenantInUse.
priority: medium
effort: s
why1: A lazy request read the tenant as absent through Acquire, and called provisioning.start only after the org's run had succeeded and left p.runs, so start found nothing and began a second Provision.
why2: start keys single flight on runs that are in progress only. It remembered failures (failed) and another process's lock (elsewhere), but nothing about a run that had just succeeded.
why3: The host reads the tenant (Acquire) and asks for a run (start) in two separate steps without one lock, so the answer from Acquire can be stale by the time start runs. The design treated a finished run as needing no memory because the next Acquire would see the tenant active.
why4: TestProvision_LazySingleFlight only exercises concurrent requests while the run is gated, so the window after the run ends was hit only by chance through the polls that follow, about once in 200 runs.
why5: Check-then-act seams between a read outside the runner's lock and a decision inside it were not pinned by a test that holds a request between the two steps.
prevention: 'start now remembers a successful run for successHold (5 s) and starts no run during it: a lazy request answers 202 provisioning, and a webhook answers 503 with Retry-After so the IdP retries and no event is acknowledged without a tenant. TestProvision_NoRerunAfterSuccess holds a request between Acquire and start while the run finishes, so the window is tested deterministically.'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

## Description

See the description property. Reproduced deterministically by
TestProvision_LazyNoRerunAfterSuccess.
