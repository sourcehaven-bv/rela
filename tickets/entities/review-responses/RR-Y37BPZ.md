---
id: RR-Y37BPZ
type: review-response
title: Hidden signal type gets a warning whose advice realtime.yaml refuses
finding: 'internal/appbuild/realtime_channels.go:61 answers CountsUnread: !mt.NoUnread for every declared type, including a persist: ephemeral signal, where NoUnread is always false. So `display: hidden` on a signal type in data-entry.yaml (internal/dataentryconfig/channels.go:352) warns "set unread: false on channel_types.<ct>.message_types.<signal>", and following that advice makes realtime.yaml fail to load with "a signal is never stored, so it never counts as unread" (internal/realtimecfg/validate.go, persist). internal/appbuild/realtime_channels_test.go:92 pins the wrong value (typing.CountsUnread true). Fix: answer CountsUnread: mt.Persist == PersistLog && !mt.NoUnread, so a signal never counts, and change the test row to expect false. Add a row to TestValidateChannels_HiddenCountsUnread for a hidden signal type with no warning.'
severity: significant
resolution: 'ADDRESSED appbuild MessageType answers CountsUnread: mt.Persist == PersistLog && !mt.NoUnread; TestRealtimeChannelSchema_MessageType expects typing.CountsUnread false and runs dataentryconfig.ValidateChannels with the real schema on a hidden signal type, expecting no warning. The row is in the appbuild test, not TestValidateChannels_HiddenCountsUnread, because that test uses fake schemas and a row there would only test the fake. MessageFacts godoc updated.'
status: addressed
---
