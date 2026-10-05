---
id: RR-SRIC1I
type: review-response
title: Mid-operation face test compared the store with itself
finding: TestFamilyRename_FaceAddedDuringRenameIsAuthorized built its expected log from the store, so a backend that stopped rolling back would still pass, and nothing asserted the family was not split.
severity: minor
resolution: The test now asserts the family either did not move or moved whole to POL-2, never split, on every backend.
status: addressed
---
