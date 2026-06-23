---
id: RR-H9QB
type: review-response
title: 'Missing test: valid body + valid If-Match + hidden id (third PATCH case)'
finding: 'TestACLWrite_PatchOnHiddenIs404 covers malformed body and stale If-Match on hidden id. The third interesting case — If-Match matching the CURRENT etag (bob obtained alice''s etag earlier, before policy change, or replayed from a log) — isn''t tested. Handler ordering says this case also 404s (gate before If-Match) but only a test pins it. Fix: third sub-case with seeded valid etag-matching If-Match, assert 404.'
severity: minor
resolution: 'Added fourth sub-case to TestACLWrite_PatchOnHiddenIs404: computes current ETag via computeETagForTest helper, asserts 404 even when If-Match would otherwise validate.'
status: addressed
---
