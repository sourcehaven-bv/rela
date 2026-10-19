---
id: RR-RSBQ9Z
type: review-response
title: Interrupting a non-follow watch reports an error
finding: "In ChannelWatchCmd.watch (internal/cli/channel_watch.go:386-391), Ctrl-C without --follow returns ctx.Err(). runKong prints \"context canceled\" and exits 1. If the interrupt lands during svc.Read (lines 369-374), the cancellation reaches channelFailed: it logs a WARN \"cli: channel operation failed\" and prints \"channel request failed\".\tA script or a person interrupts `rela channel watch` and gets exit 1 with a misleading \"channel request failed\" and a warning in the log, which looks the same as a real failure.\tCheck ctx.Err() before channelFailed whatever the value of Follow. Map an interrupt to one documented outcome: exit 130 through relaerrors.NewExitError, or nil, with no log line. Document it beside exit codes 0, 1 and 2."
severity: minor
resolution: 'An interrupt during a wait or a read no longer reaches channelFailed: --follow exits 0, otherwise exit 130 with no message and no log line. Documented next to exit codes 0, 1 and 2; test added.'
reason: 'An interrupt during a wait or a read no longer reaches channelFailed: --follow exits 0, otherwise exit 130 with no message and no log line. Documented next to exit codes 0, 1 and 2; test added.'
status: addressed
---
