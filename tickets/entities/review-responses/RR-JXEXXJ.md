---
id: RR-JXEXXJ
type: review-response
title: '`AuthorView.System` from IsReserved also flags integrations and CLI posts'
finding: '`principal.IsReserved` is also true for `integration:*` connectors and for T6 `--as system:cron` CLI posts, so `author.system` does not mean "the trigger engine". A local `--as system:realtime` can also post as the engine, which contradicts §5 "nobody can post as the trigger engine". Derive the flag from `TriggeredBy != ""` or a closed author kind, and narrow the §5 claim to remote callers.'
severity: minor
resolution: author.system is derived from TriggeredBy. T6 run-as refuses system:realtime, so the section 5 claim holds.
status: addressed
---
