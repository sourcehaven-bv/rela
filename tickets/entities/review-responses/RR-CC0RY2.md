---
id: RR-CC0RY2
type: review-response
title: Dutch error map hides the reason for conflict and forbidden errors
finding: conflict was always shown as a concurrent edit, but the server also uses it for an id that is already taken. forbidden dropped the detail that names the missing permission.
severity: significant
resolution: Removed conflict and forbidden from API_ERROR_KEYS and the locale files; only fixed-meaning codes are mapped. Tests pin that forbidden, conflict and an unknown code keep the server text under Dutch.
status: addressed
---
