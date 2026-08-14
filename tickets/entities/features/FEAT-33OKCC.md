---
id: FEAT-33OKCC
type: feature
title: Consolidate the schema file name on schema.yaml
summary: Rename metamodel.yaml to schema.yaml, resolving the two-name split against the already-shipped `rela schema` CLI and /api/v1/_schema REST surface
description: 'rela''s user-facing surface already calls this concept "schema": the CLI command is `rela schema`, the REST endpoint is /api/v1/_schema, and kong.go glosses the term as "the metamodel schema". Only the filename, the Go package, and the MCP tool get_metamodel still say metamodel. This feature consolidates the external surface on "schema", with a backward-compatible loader so existing projects keep working across a long deprecation window.'
status: implemented
---
