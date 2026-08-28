---
id: RR-D3MEV0
type: review-response
title: Docs said 'the server' warns; the warning fires wherever rela loads a policy
finding: docs/acl-security.md and GUIDE-acl-security said 'when the server loads a policy ... it logs a prominent warning at startup', but the warning fires for every appbuild consumer that loads the policy, including CLI commands — and conversely NOT for surfaces that inject their own ACL (MCP NopACL, server read-only mode), which never evaluate it. The docs were both narrower and broader than reality.
severity: minor
resolution: Both docs now say 'whenever rela loads a policy ... at server startup, and equally when a CLI command builds the full service bundle' and note that surfaces injecting their own ACL do not evaluate it and stay silent.
status: addressed
---
