---
id: RR-6PBQN3E
type: review-response
title: Preferred extension was the alphabetically first one
finding: mime.ExtensionsByType sorts alphabetically and reads the host table, so text/plain gave .conf and image/jpeg gave .jfif. attach_render shared the flaw.
severity: significant
resolution: preferredExtensions table for common types, used by mimeExtensions for both bindings. TestMIMEExtensions_PreferredFirst.
status: addressed
---
