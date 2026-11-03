---
id: TKT-GFJIMO
type: ticket
title: Align Storybook workspace versions
kind: chore
priority: low
effort: xs
status: done
---

## Description

Align the Storybook packages used across the frontend workspaces at version
10.6.2 so workspace installs load the Vue 3 Vite framework preset reliably.

## Resolution

Updated the component workspace Storybook dependencies and regenerated the
frontend lockfile. `npm ci` and the `rela-components` test suite passed.
