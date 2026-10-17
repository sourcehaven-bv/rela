---
id: RR-5TJ2OD
type: review-response
title: Reload does not consult stored type records
finding: CheckReload compares only the previous and next config. Removing a type, reloading, then re-adding it with another anchor is accepted; stored channels are read under the wrong keys and the next boot refuses.
severity: significant
resolution: RealtimeConfig.Reload now also holds the new config against the type records of every watched channel store (checkTypeRecords -> checkRecords in internal/appbuild/realtime_service.go), the same helper boot uses. Removing a type, reloading and re-adding it with another anchor or cardinality is refused once a channel of the type was written, because the record outlives the config.
status: addressed
---
