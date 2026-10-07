---
id: RR-R03TS7
type: review-response
title: Containment check fails on case-insensitive filesystems
finding: String prefix checks miss /Proj vs /proj on macOS.
severity: minor
resolution: 'Plan updated: Ancestor walk with os.SameFile; the target parent is resolved with EvalSymlinks.'
status: addressed
---
