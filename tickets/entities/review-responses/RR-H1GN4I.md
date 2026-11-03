---
id: RR-H1GN4I
type: review-response
title: pageHasMessages re-parses the tool's JSON text to detect messages
finding: tools_watch.go:540 couples the watch to read_channel's wire shape by decoding its text output. Return a typed signal from the handler path, or reuse the decoded ReadView, so a renamed field cannot silently disable the pending rule.
severity: nit
resolution: Removed it. A typed readProbe in the ctx (withReadProbe/noteRead) records the message count from the ReadView, and replay reads it.
status: addressed
---
