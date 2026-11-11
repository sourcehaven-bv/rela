---
id: RR-M55HXVZ
type: review-response
title: CLI error uses Markdown backticks and names only the web app
finding: 'internal/cli/flow_runs.go: the rela flow start error wrapped the cancel command in backticks, which print literally, and named only the web app for a retry.'
severity: nit
resolution: 'The message now reads: started X, but its first step failed (cause); retry it in the web app or over HTTP, or run "rela flow cancel X".'
status: addressed
---
