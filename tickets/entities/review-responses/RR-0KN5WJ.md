---
id: RR-0KN5WJ
type: review-response
title: busy fields are not checked by rela validate, Configure or a realtime-off boot
finding: 'Plan step 6 moves ValidateChannels'' early return. But projectsetup.channelsSection returns false when cfg.Channels is empty (internal/projectsetup/validate.go:291-296). The CLI only checks when HasChannels holds (internal/cli/validate.go:592), and Configure calls CheckChannels (cmd/rela-server/configure.go:126). So a project with a busy card field and no channels: block is never checked by rela validate or by Configure. Also, SetRealtime is not called when realtime is off (internal/dataentry/realtime_wiring.go:30-34), so the AC 8 warning for "realtime off" never appears at server boot. The plan should add projectsetup/validate.go (channelsSection, HasChannels), cli/validate.go and the configure path to Files to modify, rename HasChannels to reflect "channels or busy fields", and give AC 8 a CLI case: rela validate rejects busy: nope without a channels: block.'
severity: significant
resolution: Step 7 and Files list projectsetup/validate.go (channelsSection, HasChannels renamed HasRealtimePresentation), cli/validate.go and cmd/rela-server/configure.go, and ValidateChannels' early return now covers busy fields. Boot and reload already run ValidateChannels through SetRealtime and watcher.go, so the realtime-off warning fires; AC 8 adds CLI, Configure and SetRealtime tests.
status: addressed
---
