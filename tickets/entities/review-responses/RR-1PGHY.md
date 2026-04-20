---
id: RR-1PGHY
type: review-response
title: 'Minor: gitTimeout constant wrote out nanoseconds as a raw integer with justification ''avoids time import'''
finding: 'cranky-code-reviewer #4: `const gitTimeout = 5 * 1_000_000_000 // 5 seconds in ns; avoids time import`. The file already imports context; saving one time import for a 10-digit magic number is a bad trade.'
severity: minor
resolution: Imported time and used `5 * time.Second`. Intent is now self-evident at a glance.
status: addressed
---
