---
id: BUG-E9A72XB
type: bug
title: rela mcp fails to start when flows.yaml names roles
description: Hosts that inject a non-declarative ACL (rela mcp, rela-desktop, read-only rela-server) compile flows.yaml without the project's acl.yaml, so every role and permission in it is reported as undeclared and the boot fails.
priority: high
effort: s
why1: prepare compiled flows.yaml against resolvedPolicy(), which is nil when the injected ACL is not an *acl.Declarative (MCP and desktop inject NopACL).
why2: 'resolvedPolicy() was written for enforcement: it answers which policy the host enforces, and an allow-all host enforces none. The flows checker reused it for a configuration check.'
why3: No test booted a flows project under an injected ACL; flow tests use the default wiring, and rela flow validate loads acl.yaml itself, so it disagreed with rela mcp without any test noticing.
why4: The flows checker was added after newRealtimeConfig had solved the same problem, but nothing pointed a new config surface at that solution; each surface picked its own policy source.
why5: There was no single helper for 'the acl.yaml policy used for configuration checks', separate from 'the policy the host enforces'. loadConfigPolicy is now that helper, shared by realtime.yaml and flows.yaml.
prevention: Config checks take the project's acl.yaml regardless of the enforced ACL (flowConfigPolicy); TestFlowEngine_InjectedACLChecksFlowsAgainstACLYAML boots a flows project under NopACL and compiles an older run version.
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

## Description

`rela mcp` refuses to start on a project whose `flows.yaml` names roles,
although `rela flow validate` reports the file valid:

```
mcp startup: flows.yaml: flows.onboarding.states.collect.on.change[0].roles[0]: names role "hr", but the project has no acl.yaml; ...
```

Reproduce with the files in `internal/flowcfg/testdata` as a project.

`appbuild.prepare` compiles flows against `resolvedPolicy(injected, loaded)`.
With an injected ACL that is not `*acl.Declarative` (MCP and desktop inject
`acl.NopACL`, rela-server read-only mode injects `acl.ReadOnlyACL`), acl.yaml is
never loaded and the policy is nil. `buildFlowEngine` has the same gap when it
recompiles an older run version.
