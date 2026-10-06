---
id: FEAT-MPH34X
type: feature
title: In-app configuration editing
summary: Edit schema and screens in the app without YAML; review the draft as domain-worded changes; save with generated migrations
description: Operators edit the data model and screens in the app through a draft that is reviewed and saved; saving generates and runs any needed data migration
status: proposed
---

## Goal

Operators edit the data model and the screens of a rela project from inside the
app, without touching YAML. Edits collect in a draft, are reviewed as a list of
changes in domain terms, and are saved in one step. When a change does not fit
existing records, saving also generates and runs the data migration.

The mockups are in
`frontend/packages/rela-components/src/pages/Configure*.stories.ts` (Storybook:
"Mockups/Configure data model", "Mockups/Configure screens").

## Not this feature

Schema-as-graph (FEAT-KXV0YJ) is a separate route. This feature edits the config
files directly and does not depend on the projector.
