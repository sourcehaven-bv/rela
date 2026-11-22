---
id: BUG-JT865RH
type: bug
title: Raw not-found errors, unmarked step fields, entity wording and requests after an ended session
description: A missing entity shows "entry entity not found" with Retry and a missing list or form names the configuration, a blocked flow step marks no field and a select has no name, the discard confirm of a new record says "entity", and after a 401 the app keeps sending requests and logs their failures.
priority: medium
effort: m
why1: The view route answered a missing entry with a 422 carrying the engine's message, and the list and form wrote their own config message; the step form showed one notice and no field messages, and FieldRenderer passed no label to the select; the confirm text was a fixed string; the client sent every request whatever the session state, the stream kept reconnecting, and three catch blocks logged every failure.
why2: The not-found page of BUG-UZGMHIK covered unmatched routes only; the 422 predated it and its test pinned it as "the code it has always had".
why3: Each surface handled its own failure, so the session refusal, the not-found answer and the required marks never met one shared rule.
why4: Tests asserted the status codes and messages as they were, and no test counted requests after a refusal or checked field messages after a blocked step.
why5: There is no single place that says which answers are user states (not found, signed out) rather than faults; each caller decides, and the default is to show and log the raw error.
prevention: The view route answers a missing entry with the uniform 404 and the entity page, list and form show one NotFoundState; the API client refuses to send while a refusal is up, the stream stops, and isQuietFailure keeps those failures out of the log. Tests pin each case (Go view status, vitest per surface, e2e not-found addresses).
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in the seventh browser test of the saas-dev setup.

- A missing entity (`/entity/account/ACC-99999`) shows "entry entity not
found: ACC-99999" with Retry. A missing list shows "The list "x" does not
exist in the configuration."
- Submitting an empty flow step form says the run stays at the step and to
check the fields marked *, but no field says anything. The Country select has
no accessible name.
- The discard confirm of a new record says "This entity has not been created
yet."
- After the first 401 `session_expired` the app sends about 13 more API
requests and logs "Analyze error", "next-action load failed" and "live views:
subscription update failed".

## Acceptance Criteria

- [x] A missing entity, list or form shows the plain not-found state with a way home, without Retry.
- [x] A blocked step shows a required message at each empty required field, an unticked checkbox included.
- [x] A select is named by its field label.
- [x] The discard confirm names the type, not "entity".
- [x] After a session or workspace refusal the client sends no requests, the event stream stops, and those failures are not logged.
