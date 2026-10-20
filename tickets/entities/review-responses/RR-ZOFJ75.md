---
id: RR-ZOFJ75
type: review-response
title: 'channels: unknown-key errors come out in random order'
finding: dataentryconfig/channels.go:120-141 channelsUnknownKeys ranges over raw.Channels and messages, which are Go maps, so the error order across channel and message types changes from run to run. Users see shuffled output, and any test or golden over more than one error is flaky. Iterate sortedMapKeys as validateChannelsShape does, or sort errs before returning.
severity: minor
resolution: channelsUnknownKeys iterates sorted keys, pinned by TestChannelsUnknownKeys_Sorted.
status: addressed
---
