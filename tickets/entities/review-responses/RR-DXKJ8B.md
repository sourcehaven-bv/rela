---
id: RR-DXKJ8B
type: review-response
title: Migration re-open checks did not re-run the step
finding: After the first open user_version is current, so the second open never re-ran v8 or v9.
severity: minor
resolution: Tests stamp the old version before each open, so the second open re-runs the step over migrated data.
status: addressed
---
