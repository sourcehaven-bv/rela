---
id: RR-V5EDSSH
type: review-response
title: Scan diagnostics ignore channel file properties
finding: HasConfiguredScan and boot warnings only look at entity properties.
severity: minor
resolution: realtime.yaml reloads at runtime, so the boot-time diagnostic cannot see channel properties. Instead the first channel post whose policy has a scan command logs one warning when the command runner or sandbox is unavailable (warnIfChannelScanCannotRun); the scan itself fails closed and refuses the file.
status: addressed
---
