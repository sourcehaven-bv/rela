---
id: RR-U20G
type: review-response
title: 'F8: LoadProvider fail-soft default is wrong for script/flow entry points'
finding: For rela script and rela flow, AI is often the entire point of running the command. Silently downgrading a malformed ai.yaml to 'no provider' and logging a warning to stderr (which competes with any other output the command produces) means the user's script blows up with a not_configured error downstream and they have to dig through earlier log lines to discover the actual parse error.
severity: significant
resolution: 'Changed LoadProvider signature to (Provider, error). ErrConfigNotFound is propagated as-is for the normal ''no AI'' state. Each entry point picks its own policy: rela script and rela flow now surface non-ErrConfigNotFound errors via fmt.Errorf(''ai: %w'', err) so a malformed ai.yaml prints the parse error at startup instead of mid-script. The shared script executor and MCP lua_eval/lua_run tools keep soft-fail with slog.Warn since they run in automation/server contexts where crashing the host would be worse. Verified end-to-end: writing ''not: valid: yaml'' to /tmp/rela-ai-smoke/.rela/ai.yaml and running rela script now prints ''ai: parse /path/to/ai.yaml: yaml: mapping values are not allowed in this context''.'
status: addressed
---
