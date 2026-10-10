---
id: RR-W29XJT
type: review-response
title: Write-back can store a hash after an out-of-band version
finding: The version triggers only clear a stored hash, so a version written between the sweep's read and its write-back (row still NULL) is followed by a hash that hides the row; sqlite delete plus same-bytes re-create passes the content guard.
severity: significant
resolution: 'Write-back also requires the latest version to be a non-delete with the same hash (relations: same hash); TestSweepWriteBackSkipsARowWithANewerVersion on both backends.'
status: addressed
---
