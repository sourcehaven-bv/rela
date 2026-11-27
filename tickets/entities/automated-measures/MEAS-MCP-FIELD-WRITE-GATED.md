---
id: MEAS-MCP-FIELD-WRITE-GATED
type: automated-measure
title: MCP writes enforce field-level write grants
description: Corpus case BUG-VUNO11-mcp-update-denied-field.editor answers a denial matching REST; TestRemoteMCP_UpdateEntityHonorsFieldGrants pins the refusal through production wiring.
kind: test
location: internal/dataentry/golden_parity_test.go; internal/dataentrywire/remote_mcp_fieldgate_test.go
status: active
---

Corpus case mcp-update-denied-field.editor answers a denial, matching REST.
