---
id: RR-XZC80MB
type: review-response
title: Show in Finder reveals the previous project during Clone from Git
finding: Clone from Git clears the handler but left d.app and activePath set, so Show in Finder stayed enabled and revealed the old project.
severity: minor
resolution: focusedPath returns empty when no handler is mounted, and the clone path refreshes the menu. Covered by TestFocusedPath.
status: addressed
---
