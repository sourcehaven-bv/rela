---
id: RR-MSASEO
type: review-response
title: Undeclared-key refusal spreads to MCP and Lua
finding: Once acl.yaml has any affordance grant, undeclared keys are refused on gated surfaces.
severity: significant
reason: 'Deliberate parity with the data-entry API (F8 closure): a caller on a gated surface cannot probe or write undeclared keys. Only rela-server MCP and scheduled Lua are gated; the CLI still cleans legacy keys. Documented in acl-security.md.'
status: wont-fix
---
