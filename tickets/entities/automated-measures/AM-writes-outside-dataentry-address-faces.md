---
id: AM-writes-outside-dataentry-address-faces
type: automated-measure
title: CLI, MCP, Lua, automation and importer writes address faces
description: Delete, create_relation and import on faced types never read or create a zero-face row.
kind: test
location: internal/cli/delete_face_test.go + internal/mcp/delete_face_test.go + internal/lua/face_write_test.go + internal/entitymanager/writepaths_face_test.go + internal/entitymanager/relation_d4_test.go + internal/importer/importer_face_test.go + internal/store/pgstore/facedeleterace_test.go
status: active
---
