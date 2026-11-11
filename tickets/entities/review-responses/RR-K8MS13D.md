---
id: RR-K8MS13D
type: review-response
title: CLI and stdio MCP ran scans without the sandbox exposure check
finding: The command runner was reachable from CLI and stdio MCP assemblies, which skip the data-entry server's check that the sandbox does not expose the project.
severity: significant
resolution: The runner sits in an atomic holder on SharedBase. Only dataentrywire installs it through SetChannelFileRunner; other assemblies refuse files that need a command and warn once. Tested in TestSetChannelFileRunner.
status: addressed
---
