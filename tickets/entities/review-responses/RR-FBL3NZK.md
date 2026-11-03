---
id: RR-FBL3NZK
type: review-response
title: Journal rotation needs a lock and a size cap
finding: Two writers can rotate at once; rename fails on Windows with an open handle; inode reuse defeats os.SameFile alone.
severity: minor
resolution: Rotation under the store file lock; 4 MiB hard cap truncates when rename fails; shrink check and line validation kept; failed-rename path tested with an injected error (no Windows test runner in CI).
status: addressed
---
