---
id: RR-CPBMJEU
type: review-response
title: "App Info depended on the Configure space"
finding: "The card was gated on can_configure, which needs the Configure space, so on a server without it no one saw App Info, admins included."
severity: significant
resolution: "The card is gated on can_edit_settings, the permission that already shows the shared settings. It stays a display choice: the counts come from the schema every user loads."
status: addressed
---
