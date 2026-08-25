---
id: DOCS-Q4MB31
type: docs-checklist
title: 'Docs: Framework-level loading/pending indicator system for the data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc / module comments on new exported surfaces
- [x] Non-obvious decisions explained at the point of the decision
- [x] Comments state what is true (no claims the code does not honour)

`useDelayedPending`, `useNavigationPending`, `pendingTimings`, `PendingButton`
and `ActivityBar` each carry a header explaining the *decision*, not the
mechanics — why identity-tracking rather than a counter, why a label swap rather
than a spinner, why delay scales with invasiveness.

The code review found three comments asserting properties the code did not have.
All three are corrected, and that class of defect is the reason several of the
fixes are comment changes rather than code changes:

- `pending.css` claimed the ten `@keyframes spin` copies were
"byte-identical except for RelationCards' 0.6s". False — RelationCards'
re-applied `translateY(-50%)`, and that false claim is why the regression
shipped (RR-U186F1).
- `pending.css` claimed its reduced-motion list covered "every rotating
busy affordance". It could not reach any scoped one (RR-RQLNJ6).
- `useDelayedPending` justified `flush: 'sync'` with a race that Vue's
scheduler does not permit. It is load-bearing for a *different* reason —
pre-flush coalescing merges two operations into one display period — which was
verified by probe and is now what the comment says.

## Project Documentation

- [x] `frontend/CLAUDE.md` — new "Pending and loading indicators" section
- [x] Conventions written as rules a future contributor can follow

Covers: the three-class table with timings; the governing rule; the invasiveness
principle; the ban on in-button spinners and skeletons; the keep-previous-data
rule (including the `isPending` vs `isFetching` distinction and `DocumentView`'s
`!docContent` guard); one-indicator-per-act; the `aria-disabled`-primary-only
accessibility rule; both sanctioned exceptions (`ConfirmModal`, and
`useAutoSave`'s debounce-is-the-delay); and the scoped-CSS rule — a new spinner
needs its own reduced-motion rule unless it is unscoped, because a `[data-v-*]`
selector outranks anything `pending.css` can write.

The testing note records why an e2e probe element built with `createElement`
only picks up unscoped CSS — the thing that made the reduced-motion gap
invisible in the first place.

## External Documentation

- [x] `docs/data-entry.md` — user-facing note under Overview
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel surface)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface)
- [x] ~~Root `CLAUDE.md`~~ (N/A: frontend-local convention)
- [x] ~~`README.md`~~ (N/A: no project-level change)

The user note exists mainly to answer one question before it is asked: *why do I
see no spinner?* Under this design a fast operation shows nothing at all, which
is easy to mistake for a missing feature. It describes what each of the three
indicators looks like and states plainly that an absent indicator means the
operation was quick.
