---
id: RR-ZHUB7AY
type: review-response
title: Records HTTP handler carries the Lua appender
finding: recordsHandler stores a lua.RecordAppender it only passes on, and SetRecords takes a service and an appender that could mismatch.
severity: minor
reason: Both values come from the same service at the one wiring site (dataentrywire). Removing the pair needs the adapter in a package both appbuild and dataentry can import; that is a refactor beyond this ticket.
status: deferred
---
