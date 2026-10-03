---
id: TKT-3RDMDD
type: ticket
title: Data-entry, templates and scripts read config through the services' config loader
kind: enhancement
priority: high
effort: l
status: ready
---

## Description

internal/dataentry builds its own config.FSLoader and FSTemplater, and scripts/,
actions/, validations/, apps/ and custom/ are read with os.OpenRoot. None of it
sees config baked into rela.db. Route each through the config.Loader the
services were assembled with.

## Acceptance criteria

- A project with config only in rela.db serves the data-entry app, runs automations, actions, validations and scheduled scripts, applies entity templates and serves custom/ and apps/ assets.
- Path containment is preserved on both backends.
