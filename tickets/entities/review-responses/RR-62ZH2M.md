---
id: RR-62ZH2M
type: review-response
title: Membership warning fires on all appbuild consumers, not only rela-server
finding: TKT-T31NKT scope item 2 says "rela-server logs the ungated-membership condition as a prominent startup warning", but the implementation places warnUngatedMembership in appbuild.buildACL, which is also reached by ordinary CLI commands (internal/cli/kong.go:170, flow.go:43, validate.go:83). Every `rela` command run against an ungated policy prints the warning line, not just server startup.
severity: minor
reason: 'Deliberate: warnUngatedMembership sits in the shared buildACL helper (internal/appbuild/appbuild.go:614), the single place every recipe resolves a real policy, so the predicate has exactly one call site and CLI/server cannot drift. The cost is one stderr slog line per CLI invocation against an open policy; the benefit is that operators editing acl.yaml locally see it without running the server. Narrowing to server-only would fragment the build-agnostic wiring (per appbuild''s prepare/assemble rule) for no security gain.'
status: wont-fix
---
