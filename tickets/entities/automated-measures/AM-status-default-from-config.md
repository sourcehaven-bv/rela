---
id: AM-status-default-from-config
type: automated-measure
title: A create without status sets one only from a declared schema default
description: Creates entities through the entity manager, import, MCP, Lua, the HTTP API and a web form, and generates templates. Asserts that status is set only from a declared default (property default, type initial, type default) and stays absent otherwise.
kind: test
location: internal/entitymanager/default_status_test.go; internal/importer/importer_test.go; internal/mcp/default_status_test.go; internal/dataentry/default_status_test.go; internal/metamodel/types_test.go; internal/templating/fsloader_test.go; e2e/tests/create-default-status.spec.ts
status: active
---
