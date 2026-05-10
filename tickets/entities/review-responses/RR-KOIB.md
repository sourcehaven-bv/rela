---
id: RR-KOIB
type: review-response
title: loadUserLogo named results add no value
finding: 'theme_logo.go:88-121 declares named return values but every return statement is explicit, so the names are dead noise. Drop them: `func (a *App) loadUserLogo() ([]byte, string, error)`.'
severity: minor
resolution: Dropped the named return values from loadUserLogo. The gocritic linter then re-flagged it (it prefers names for triple returns), so added `//nolint:gocritic` with a one-line justification. Reviewer's preference wins for readability.
status: addressed
---
