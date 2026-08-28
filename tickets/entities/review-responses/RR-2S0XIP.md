---
id: RR-2S0XIP
type: review-response
title: Overlapping refetches could settle on a stale window
finding: refetchGrid published whichever fetch resolved last. Navigating periods quickly could leave the grid showing an older window's events while the period label read the newer one — with no error and nothing in the UI admitting the mismatch. Superseded fetches also kept paging to completion.
severity: significant
resolution: 'Generation counter plus AbortController: results publish only if still current; the previous request is aborted; an abort is not reported as a load error. Pinned by a test verified to fail without the guard.'
status: addressed
---

## Finding

`refetchGrid` published whatever resolved last. Navigating periods quickly
starts overlapping fetches, so clicking "next" twice could settle on the
**first** window's events while the period label reads the second.

Nothing in the UI admits the mismatch — no error, no spinner, just a grid
showing the wrong month. Self-found while reviewing the new fetch path.

A second, smaller problem in the same function: a superseded fetch kept paging
to completion, doing work whose result was discarded.

## Resolution

A generation counter plus an `AbortController`:

- Each call takes a sequence number; results are published only if that
number is still current, so a stale window can never overwrite a newer one.
- The previous request is aborted, so a superseded fetch stops paging.
- An abort is not reported as a load error — it is the component superseding
itself, not a failure.

Added `CalendarView.test.ts` → "CalendarView refetch races", which holds the
August response open, navigates to September, then releases August and asserts
the grid shows September. Verified it FAILS without the generation guard.

## Test-harness defect found on the way

The router mock's `replace()` was a bare `vi.fn()` that swallowed the write, so
`routeQuery` never changed. Since view and date live in the URL, the component
was frozen on its initial period — and the existing navigation tests were
passing without actually exercising the URL round-trip. The mock now updates the
query like a real router.
