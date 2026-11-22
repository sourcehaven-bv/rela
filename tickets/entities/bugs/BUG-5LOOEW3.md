---
id: BUG-5LOOEW3
type: bug
title: Tenant pages keep fetching, fixed read-only text and jargon in labels
description: A suspended or archived workspace page has the app name as tab title and logs 5 to 6 refused requests; the read-only banner text cannot be changed; the account menu says "Switch org" and the editor placeholder says "entity".
priority: medium
effort: s
why1: The app sent its schema, config, dashboard, root-route and event-stream requests at start and learned the workspace state from their 403 answers; the access screen set no page title; the banner text was a fixed i18n string; two labels used short or internal words.
why2: The tenant status endpoint, which answers a suspended or archived workspace with 200, was read only after the schema had loaded, and only for the read-only flag.
why3: The access screens were added after the start-up sequence and were driven by refusals of ordinary requests, not by the status that already said the same thing.
why4: Tests checked the screen and the flag after a refusal, but none counted the requests a blocked workspace receives at start.
why5: The start-up order has no step that asks whether the workspace can be used at all before loading it.
prevention: main.ts reads the tenant status before it installs the router and mounts the app, and App.vue loads nothing while a page replaces the app. Unit tests pin the status handling, the tab titles and the banner text from the shell.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in the eighth browser test of the saas-dev setup (pool and harbourmaster).

- A suspended or archived workspace page has only the app name as tab title,
and the console shows 5 to 6 "403 Forbidden" errors.
- The read-only banner always says "This workspace is read-only. Creating and
editing are turned off."
- The account menu says "Switch org".
- The note editor placeholder says "Write, or type @ to link an entity".

The dashboard intro on a phone is fixed by BUG-UX4P9T2.

## Acceptance Criteria

- [x] A suspended, archived or unknown workspace page names the tab, such as "Workspace suspended · Sourcehaven Workspace".
- [x] A suspended or archived workspace gets no request it refuses at start, apart from the event stream that BUG-JT865RH stops.
- [x] `account.read_only_text` replaces the read-only banner text; without it the default stays.
- [x] The account menu says "Switch organisation".
- [x] The editor placeholder says "Write, or type @ to link a record".
