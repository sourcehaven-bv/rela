---
id: BUG-21N3J4N
type: bug
title: The tenant setting-up screen ignores the app language and name
description: On a multi-tenant server the setting-up screen follows the browser language instead of app.language and its tab title is rela instead of app.name.
priority: medium
effort: s
why1: 'The host serves the setting-up screen from StaticShell, which holds only the built index.html: title rela and lang en, and no app config.'
why2: The SPA learns app.name and app.language from its config request, which a tenant that does not exist yet cannot answer, so it falls back to the browser language and the name rela.
why3: StaticShell was designed to read no project file so it cannot leak tenant data, and the shared operator config was left out with it.
why4: No rule separated operator config, which is not secret and is the same for every tenant, from tenant data in the tenant-free path.
why5: Pre-tenant screens were tested for safety, not for how they present the app.
prevention: Tests pin that the tenant-free shell carries the shared app name and language and that the SPA uses them before its config loads.
started: "2026-11-20"
completed: "2026-11-21"
status: done
---

## Description

A multi-tenant server with lazy provisioning shows its "setting up" screen for 5
to 8 seconds on an org's first request. The host serves that page from the
embedded shell without any project config. So the screen follows the browser
language (Dutch in the report) instead of `app.language`, and the tab title is
"rela" instead of `app.name` (a project's `custom/custom.js` does not run there
either).

## Acceptance Criteria

- [x] The setting-up screen uses the shared project's `app.language` when the user has chosen no language.
- [x] Its tab title is the shared project's `app.name`.
- [x] Values are escaped; a project without `app:` keeps today's behaviour.
- [x] The behaviour is documented.
