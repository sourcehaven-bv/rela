---
id: RR-GG24NR
type: review-response
title: Commit continues after unmount
finding: A commit finishing after the view unmounted reloaded and toasted on another page.
severity: minor
resolution: An alive flag stops the write, reload and messages after unmount. Test added.
status: addressed
---
