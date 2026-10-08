---
id: RR-FUW561
type: review-response
title: 'Design: CLI applies read paths before logging is configured and warns on every command'
finding: internal/cli/kong.go calls SetHostReadOnly before configureKongLogging, so its warnings bypass --quiet and the configured handler, and the missing-path warning fires on every rela command (list, show, ...) on a host whose clamd socket does not exist yet.
severity: minor
resolution: The CLI calls ApplyHostEnv after configureKongLogging, so warnings honour --quiet. SetHostReadOnly no longer warns about missing paths at all (bwrap -try skips them); only rela-server reports a missing path at startup, so ordinary rela list/show runs stay quiet.
status: addressed
---
