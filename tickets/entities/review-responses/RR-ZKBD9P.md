---
id: RR-ZKBD9P
type: review-response
title: Prompt-injection mitigation (client_baselines) does nothing on stdio MCP
finding: Risk 1 relies on operators restricting agents through client_baselines, which key on a verified principal_type. Stdio rela mcp is wired with NopACL (mcp_wiring.go), so openAccess holds every permission, moderator() is true, and no principal_type exists. An injected message can make a stdio agent remove or post in any channel. Either wire the stdio channel service with acl.yaml like the CLI (decision 9), so a run-as agent identity can be limited, or state the residual risk plainly in docs/realtime.md and the mcp docs.
severity: significant
resolution: 'Per Jeroen: stdio MCP channel tools and CLI channel commands apply acl.yaml when present, so a run-as agent identity can be limited. Docs state that locally this is not a security boundary.'
status: addressed
---
