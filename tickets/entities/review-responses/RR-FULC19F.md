---
id: RR-FULC19F
type: review-response
title: Concurrent-process test never rotated
finding: TestHintFeed_ConcurrentProcesses wrote about 250 KB, below the 1 MiB rotation size, so rotation racing with appenders was untested.
severity: minor
resolution: The feed's rotation size is a field; the helper process takes it from the test. A rotating case (16 KiB) runs four processes and checks both journal files for torn or repeated lines and that rotation happened.
status: addressed
---
