---
id: RR-DL1ZWO3
type: review-response
title: Renames test only the new location
finding: diff --name-only lists only the rename destination.
severity: minor
resolution: Added --no-renames so both sides are listed; a deleted source directory is skipped by the -d check.
status: addressed
---
