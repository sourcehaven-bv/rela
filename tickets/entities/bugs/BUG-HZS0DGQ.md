---
id: BUG-HZS0DGQ
type: bug
title: The tenant setting-up screen still ignores the app language
description: 'The pool tenant setting-up screen shows Dutch text for a Dutch browser although data-entry.yaml sets app.language: en. The tab title also falls back to rela.'
priority: medium
effort: xs
why1: 'App.vue watched schemaStore.app?.language with immediate: true, so at mount it called setProjectLanguage(undefined) and cleared the language the server wrote into the shell. The title read schemaStore.app?.name, which is the placeholder rela before the config loads, so the shell''s name never applied.'
why2: The schema store's app is a placeholder object before /_config loads, and nothing marks it as not loaded; the callers read it as if it were the config.
why3: The BUG-21N3J4N tests checked bootMeta and locale.ts in isolation by importing the module fresh. No test ran the App shell wiring that runs right after.
why4: The fix was verified on the server output (the built shell) and in unit tests, not on the live setting-up screen with a non-English browser.
why5: Values that hold until a later load have no single owner in the SPA; each caller decides how to merge them with the store's placeholder.
prevention: useShellAppConfig owns the merge of the shell's boot values with the store and only reads the store once it has loaded. Its test mounts the wiring over a not-yet-loaded store and fails if the boot language is cleared. frontend/CLAUDE.md says to read schemaStore.app only once loaded.
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

BUG-21N3J4N made the server write `app.name` and `app.language` into the
tenant-free shell. The server side works: the pool shell carries `<html
lang="en">`, `<title>Sourcehaven Workspace</title>` and both `rela-*` meta tags.
The SPA then drops both values when it mounts. App.vue watches
`schemaStore.app?.language` with `immediate: true`, so before the config loads
it calls `setProjectLanguage(undefined)` and clears the language from the shell;
the browser language wins. The title reads `schemaStore.app?.name ??
bootMeta(...)`, but the store's placeholder app is `{ name: 'rela' }`, so the
shell's name is never used.

## Acceptance Criteria

- [x] Before the config loads, the SPA keeps the language and app name the server wrote into the shell.
- [x] Once the config loads, `app.language` and `app.name` from the config apply as before.
- [x] A test mounts the shell wiring, not only the module, so the regression is caught.
