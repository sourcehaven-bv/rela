---
id: RR-EFVZES7
type: review-response
title: openTenant error path leaks the relay
finding: When buildApp failed the new CollabRelay was not shut down.
severity: minor
resolution: The error path now shuts the relay down with the shutdown grace.
status: addressed
---
