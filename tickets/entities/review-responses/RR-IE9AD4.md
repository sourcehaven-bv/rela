---
id: RR-IE9AD4
type: review-response
title: Untested parity paths on MCP
finding: config_reloaded, an unbound action with an unreadable entity, and gate errors had no MCP test.
severity: significant
resolution: Added TestMCPActions_ConfigReloaded and TestMCPActions_UnboundRunsWithoutUnreadableEntity. The gate-error path shares failGate with the comment tools, which their tests cover.
status: addressed
---
