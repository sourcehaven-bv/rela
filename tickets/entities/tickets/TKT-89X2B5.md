---
id: TKT-89X2B5
type: ticket
title: 'rela-docs phase 3 (Tier B): screenshot{} island — chromedp capture of the seeded data-entry SPA'
kind: enhancement
priority: medium
effort: l
status: review
---

Phase 3 of FEAT-G4VO53. Adds the **`screenshot{}` island** to the doc language
(phase 2, TKT-3RLZR4): a metamodel-driven headless-Chrome capture of the
data-entry SPA rendering a seeded entity, embedded in the manual as
`![](figure.png)`.

## What

A `screenshot{}` statement-island resolver that:
1. stands up an **in-process data-entry server** backed by the build's **seeded memstore** (same one `create`/`link` populate for `entity{}`/`graph{}`),
2. drives **headless Chrome via chromedp** (already a dependency, v0.16.0 — no new dep) to the relevant SPA view (a form/edit page for a seeded entity, a list, etc.),
3. captures a clipped PNG (optionally annotated with **arrows-with-text** anchored to schema fields), writes it beside the output, and emits a Markdown image reference.

## Decisions (confirmed with user)

- **chromedp** (Go-native, single-language, CI-runnable). Explore/spike first; flag if friction is high.
- **NO graceful degradation.** If there is no browser, `screenshot{}` **errors loud** (a `BuildError`), exactly like any other fail-loud resolver. No placeholder, no build-succeeds-without-figures path. One code path.
- **Base:** stacked on the phase-2 branch (`tkt-3rlzr4-doc-language`); rebase onto develop after PR #1181 merges.
- Reuse the **`internal/dataentry/e2e_test.go`** chromedp pattern (it already drives the SPA headless).

## Anchoring & annotations (carried from the design)

- `screenshot{ view="form", type=..., entity=r.id, as="role", arrows={{at="field", text="..."}}, out="fig.png" }`.
- `at` targets a **schema field** via a stable SPA hook (`data-field="<name>"` if present — else add one) → fail-loud if the field doesn't exist; or `@button:`/`@role:` → ARIA. openvwr's arrow/box/badge vocab, arrow-with-text primary.
- Annotations drawn via an injected overlay (chromedp `Evaluate` of an annotate script) OR post-capture Go image compositing — decide in planning.

## Open questions for planning

1. **Server wiring** — the cheapest way to build dataentry.App's 10 collaborators over a memstore without pulling bleve into production (appbuildtest is test-only). May need a small production memory-wiring helper, or a `//go:build` seam.
2. **SPA assets** — are the Vue static assets embedded in the binary (servable in-process) or do they need a build? `screenshot{}` needs the SPA to actually load.
3. **Chrome discovery + CI** — chromedp default resolution; skip/error when absent; is a browser available in CI (the e2e job)? This island's tests are browser-gated.
4. **Role-scoped capture** (`as=`) — how to make the in-process server render as a given ACL role (a fake principal/session).
5. **Annotation mechanism** — DOM overlay vs Go image compositing; injection-safe text.
6. **Where the harness lives** — `internal/docs/screenshot.go`? A separate `internal/docscapture` to keep the browser dep off the core `internal/docs`?

Spike the chromedp+memstore-server path end-to-end BEFORE committing the full
plan (per user: discuss if a lot of issues surface). Design in RES-EK7LSA
addendum "Tier B / phase 3".
