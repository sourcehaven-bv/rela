---
id: FEAT-DO57
type: feature
title: Server-side actions in data-entry
description: Add user-triggered actions to the data-entry app. Actions are named operations defined in data-entry.yaml that execute Lua scripts server-side and return a response (redirect URL or toast message). Navigation entries can reference actions to provide buttons like 'Today's Note' that find-or-create an entity and redirect to it. Scripts live in actions/ and can receive static params from the config, allowing one script to serve multiple nav entries with different configurations.
status: proposed
---
