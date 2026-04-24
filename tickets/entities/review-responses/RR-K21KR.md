---
id: RR-K21KR
type: review-response
title: Fragile Location-header slicing in dataentry test
finding: TestV1CreateEntity_SavesRelations parsed the created ID from the Location header via loc[strings.LastIndex(loc, '/')+1:]. If Location lacked a '/' (LastIndex -> -1), the full string would pass the non-empty check and be treated as an ID.
severity: significant
resolution: The test now decodes the JSON response body into V1Entity and reads created.ID directly. The body is the authoritative payload; header parsing is no longer in the test path.
status: addressed
---
