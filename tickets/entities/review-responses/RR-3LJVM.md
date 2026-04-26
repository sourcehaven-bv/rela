---
id: RR-3LJVM
type: review-response
title: 'F2: rela-server is unauthenticated; envelope leakage on --bind 0.0.0.0'
finding: cmd/rela-server/main.go binds 127.0.0.1 by default but allows --bind 0.0.0.0 with no auth layer. The plan's risk-table assertion that 'action/document UI is authenticated' is factually wrong for rela-server (and rela-desktop, which embeds the same handler). After this change a LAN-bound deployment would leak full Lua source slices, captured print() output, redacted-by-key-only args, and full Lua stack traces to anyone on the network. The threat-model framing needs correction and either degraded envelope on non-loopback or an opt-in flag.
severity: critical
resolution: 'Threat-model framing corrected in Security section. Added AC #9: rich envelope detail (source, captured_output, stack) is loopback-gated by default; opt-in via data-entry.script_errors.full=true. Added internal/dataentry/script_errors.go with resolveScriptErrorConfig helper using net.IP.IsLoopback. Startup log announces the choice.'
status: addressed
---
