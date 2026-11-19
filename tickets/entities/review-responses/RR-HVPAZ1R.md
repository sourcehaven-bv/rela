---
id: RR-HVPAZ1R
type: review-response
title: builtins listed as protective in the configedit allowlist
finding: spaces.[].builtins.** was in the protective list, which is for keys that restrict. builtins is a UX filter that protects nothing, and the classification could be read as a security property. Its removal behaviour was also untested.
severity: minor
resolution: Removed from protective. Both trees stay locked (not in the Configure space yet), so Configure cannot write them.
status: addressed
---
