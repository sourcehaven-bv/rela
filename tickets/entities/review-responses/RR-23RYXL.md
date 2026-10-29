---
id: RR-23RYXL
type: review-response
title: Test file names that do not exist
finding: There is no internal/realtimecfg/keys_test.go; key-walk tests live in load_test.go (for example TestParse_BoolKeysUnchanged, load_test.go:920). There is no RlChannelComposer unit test, only RlChannelComposer.stories.ts. The composer is mounted in frontend/src/components/entity/ChannelsPanel.test.ts. Name the real homes, or say the files are new.
severity: nit
resolution: Replaced keys_test with load_test.go and the composer unit test with ChannelsPanel.test.ts and RlChannelComposer.stories.ts.
status: addressed
---
