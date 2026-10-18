---
id: RR-WOAZJU
type: review-response
title: rela-desktop is not covered by the RELA_RUN_AS rule
finding: Section 0 says only the CLI and stdio MCP honour it and that rela-server refuses it. cmd/rela-desktop is not mentioned; state whether it ignores or refuses it.
severity: nit
resolution: rela-desktop refuses RELA_RUN_AS like rela-server.
status: addressed
---
