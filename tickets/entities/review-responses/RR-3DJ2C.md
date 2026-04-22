---
id: RR-3DJ2C
type: review-response
title: Cleanup errors swallowed hide test pollution
finding: 'fs.rmSync errors are caught and ignored after force: true. A thrown error there means a real problem (open fd, wrong perm) and should log a warning instead of being silenced.'
severity: significant
resolution: testProject fixture teardown now console.warns with path + error on rmSync failure instead of silently ignoring.
status: addressed
---
