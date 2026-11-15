---
id: RR-EHO9WSE
type: review-response
title: Misspelled top-level remote_mcp key is ignored
finding: Only keys inside remote_mcp were strict; remote-mcp or remoteMCP at the top level was dropped silently, losing an enabled false.
severity: significant
resolution: refuseMisspelledRemoteMCP refuses any top-level key that normalises to remote_mcp but is spelled differently; tested.
status: addressed
---
