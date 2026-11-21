---
id: BUG-UZGMHIK
type: bug
title: Entity page refresh after actions, step scroll, sidebar flash, run actor, unknown routes and 422 log
description: Six frontend defects found in a browser test of a local SaaS environment. The Changes list kept its old rows after an action, "Open step" left the step form at the bottom edge, the sidebar showed generic rows before the space's navigation, the run timeline showed a raw principal, unknown addresses and /login rendered an empty pane, and a 422 refusal of a create form was logged as an error.
priority: medium
effort: m
why1: The panels below the entity view read their own lists only when their props change, and an action changed none. The step form was scrolled once, before the panels above it had rendered. The sidebar drew its built-in rows before the sidebar answer arrived. The timeline used the principal string when the server sent no title. The router had no catch-all route and no /login route. The create form's catch logged every failure, including a handled validation refusal.
why2: Each of these was written for the state the page is in once everything has loaded, with a fast local server and a user who navigates only through the app's own links.
why3: The tests mount one component at a time with answers that arrive at once, so they never saw a panel go stale after an action, content shift above a scroll target, or rows drawn before their config. No test resolved an unknown path.
why4: The behaviour across a whole page (an action, then every section on it; a deep link, then the settled layout) had no owner and no test that looked at the page as a user sees it.
why5: The SPA was first run behind a hosted SaaS edge, with real latency, a signup bot and links from outside the app, in this browser test; the defects only show in that setting.
prevention: Vitest tests pin that an action bumps the panel revision and the Changes list reads again in place, that keepInView re-scrolls when the target moves and stops when the user scrolls, that the sidebar draws no rows before its answer, that a system principal reads "System" in the timeline, that unknown paths with and without a space resolve to the not-found page and /login to the home of its space, and that a 422 refusal logs nothing. The flow task hint e2e test checks that the step form's top is in view.
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

A browser test of the local SaaS environment (pool-rela behind the pratique
edge, harbourmaster through the hub) found six defects in the SPA.

1. After an action on an entity page, the Changes list shows the old rows
   until a reload. The view itself reloads, the panels below it do not.
2. "Open step" (a flow task hint, `?run=`) scrolls the step form to the
   bottom edge of the view.
3. On a full page load the sidebar shows Search, Flow tasks, Flow runs and
   Analysis for a moment before the space's own navigation.
4. The flow run timeline shows `system:signup-bot`, where Changes says
   "System".
5. `/s/backoffice/runs` renders an empty page. `/login` with a session lands
   on `/s/portal/login`, also empty.
6. A 422 validation refusal is logged as a console error.

## Analysis

- The step form itself does not log a refusal. The 422 a browser reports as
  "Failed to load resource" is the browser's own network log, which the app
  cannot suppress. The create form (`DynamicForm`) did log a handled
  refusal with `console.error`; it now logs only unexpected failures.
- The signup bot's principal is not a person entity, so the server sends no
  `by_title` for it; the timeline now shows "System" for it, as Changes does.

## Acceptance Criteria

- [x] After an action or an applied proposal, the Changes list, documents, flow runs and "Mentioned by" read again without a reload.
- [x] `?run=` keeps the step form at the top of the pane, below the sticky bar, until the page settles or the user scrolls.
- [x] The sidebar draws no nav rows until the sidebar answer arrives; a failed load still shows the built-in rows.
- [x] The run timeline names a system principal "System", and a person by title.
- [x] An unknown address, with or without a space, shows "Page not found" with a link home (en and nl); `/login` opens the home page, and `/s/<space>/login` the home of that space.
- [x] A 422 refusal of the create form or the step form logs nothing to the console.
- [x] The behaviour is documented.
