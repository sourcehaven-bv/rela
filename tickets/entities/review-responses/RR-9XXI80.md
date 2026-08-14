---
id: RR-9XXI80
type: review-response
title: Deprecation warning is unreachable for the server and MCP entry points
finding: 'The plan puts the one-shot warning ''at the CLI entry point, driven by a Context field'' and justifies excluding Discover() because it runs per-request. Both halves are right, but the result is that users who ONLY run rela-server, rela-desktop, or the MCP server never see the deprecation notice at all — those binaries do not go through the CLI command entry the warning is attached to (appbuild.Discover at appbuild.go:688 is the shared path, and the desktop app calls project.Discover directly at main.go:797). Since the whole point of the deprecation window is to get users to migrate before a major version drops the legacy name, silently never telling a server-only or desktop-only user is a design gap. The fix is cheap and does not reintroduce log spam: emit once at PROCESS STARTUP (where each binary constructs its Services / opens a project), not per request — a sync.Once or a single log line in the startup path of each entry point. Recommend restating the rule as ''once per process at startup, in every entry point'' rather than ''at the CLI entry point''.'
severity: minor
resolution: Plan updated. The rule was restated from 'once at the CLI entry point' to 'once per process at startup, in every entry point' — approach step 4 now places it in the shared appbuild.Discover path plus the desktop app's direct project.Discover call, guarded by sync.Once. This keeps the original justification for excluding project.Discover() itself (per-request in server contexts) while ensuring server-only, MCP-only and desktop-only users actually see the notice before the legacy name is dropped. AC-9 updated to name all four binaries.
status: addressed
---
