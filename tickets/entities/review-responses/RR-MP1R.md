---
id: RR-MP1R
type: review-response
title: Type the import response struct
finding: handlers_theme_package.go:107-113 — Returns untyped map[string]any with field name `palette`, while the parallel _settings handler uses a typed APISettingsData with `userPalette`. Different field name and untyped means future renames go uncaught. Define APIThemeImportResponse with json tags and use it; matches what the frontend already does on its side.
severity: minor
resolution: Added APIThemeImportResponse struct with json tags `palette` and `logoUrl,omitempty`. handleAPIThemeImport now writes the typed struct rather than a map[string]any. Tests still pass; future field renames will fail at compile time.
status: addressed
---
