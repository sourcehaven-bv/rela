---
id: RR-9IP4A3
type: review-response
title: Syslog dial happens late in startup
finding: wireAccessLog ran after project discovery and store open, so an unusable destination failed after side effects; exit 2 is for usage errors.
severity: minor
resolution: openAccessLog runs right after configureLogging, before discoverProject, and exits 1; flag validation stays exit 2.
status: addressed
---
