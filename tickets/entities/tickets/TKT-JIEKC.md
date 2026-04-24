---
id: TKT-JIEKC
type: ticket
title: Honor return_to as a back affordance on non-form screens
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

`return_to` is consumed only by `DynamicForm` (create + edit). Every other
screen the SPA renders ignores it. But the document link rewriter is a general
mechanism for "the user clicked through from context X; send them back there on
dismiss." Today, if a document link targets e.g. `/entity/ticket/TKT-001`, the
user has no path back to the document except browser back — and no visible
affordance.

This is a UX hole in the feature TKT-4MFUK shipped: we promised documents become
a navigational hub, then only wired the back trip on form submits.

## Scope

**In scope**

1. Extend the document link rewriter to append `return_to` on non-form internal links too (entity detail, list, view, kanban, document, search, analyze, dashboard). Form routes keep the current behaviour — id-anchor + return_to.
2. Introduce a shared `<ReturnToBanner>` component (or equivalent header affordance) that reads `?return_to=` from the current route and renders a "← Back to …" button routing via vue-router.
3. Wire the banner into screens reachable from document links that don't already have their own cancel/back button:
   - EntityView (detail)
   - ListView
   - CustomView
   - KanbanView
   - DocumentView (replaces the existing bespoke `goBack()` / `?from=` wiring)
   - SearchView
   - AnalyzeView
4. Safety: reuse the existing `isSafeReturnPath` guard on the receiving side so an attacker-planted `?return_to=//evil.com` is rejected loudly, not rendered.
5. Replace DocumentView's `?from=<list-id>` mechanism entirely — it's a pre-`return_to` workaround that's now redundant.

**Out of scope**

- Dashboard / Settings / Conflicts screens (not a typical target of document links; can be added later if a use case appears).
- The back-label text — default is "← Back" unless the originating URL encodes a title; richer labels are a follow-on.
- Multi-hop stacks (`return_to` chains). Single-hop only; deeper navigation should use browser back.
- Form screens (already handled by DynamicForm; unchanged).

## Why now

The mechanism is in place, the rewriter is half-done, and the gap is visible to
anyone testing the new `rela.url.*` helpers that target non-form routes
(`detail`, `list`, `view`, `kanban`, `document`, `search`, etc.). Shipping those
helpers without a back path makes them feel worse than the original `create://`
/ `edit://` schemes they replaced.
