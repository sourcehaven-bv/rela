---
id: RR-E2IYVG
type: review-response
title: Other ChannelSchema implementations are not listed
finding: Adding MessageFacts breaks every implementation of dataentryconfig.ChannelSchema, not only the fake in channels_test.go. internal/dataentry/realtime_wiring_test.go:46 (roomSchema) and internal/projectsetup/validate_test.go:316 (noChannelTypes) also implement it and are not in Files to modify.
severity: minor
resolution: ADDRESSED roomSchema (dataentry/realtime_wiring_test.go) and noChannelTypes (projectsetup/validate_test.go) listed in Approach 4 and Files to modify.
status: addressed
---
