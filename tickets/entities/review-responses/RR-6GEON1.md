---
id: RR-6GEON1
type: review-response
title: Invariant test is a blocklist
finding: TestSystemReadOnlyPathsHoldOnlyBinariesAndLibraries passes with /mnt, /media, /boot, /sys or /usr/../etc added. Assert exact equality with the six system paths.
severity: minor
resolution: Renamed to TestSystemReadOnlyPathsAreFixed and asserts exact equality with the six system paths, so any change to the list must also change the test.
status: addressed
---
