---
id: RR-BJ8T6A
type: review-response
title: HistoryReader skips the Addressable gate
finding: ListVersions with a zero or malformed ref reaches SQL.
severity: minor
resolution: HistoryReader godoc states an unaddressable ref has no history; pinned by storetest FaceLineage/UnaddressableRefHasNoHistory on both database backends.
status: addressed
---
