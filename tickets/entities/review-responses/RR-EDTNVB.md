---
id: RR-EDTNVB
type: review-response
title: Nothing writes the type record yet
finding: RecordTypes has no production caller, so the boot refusal cannot fire until the service lands.
severity: minor
resolution: 'The channel service writes the record: realtime.Service.record (internal/realtime/service.go) calls Store.RecordTypes on the first channel write of each type. Records are insert-if-absent, and a conflicting shape returns ErrTypeShapeConflict, so the boot refusal in checkChannelTypeRecords now fires against real records.'
status: addressed
---
