---
id: RR-NY7PRB
type: review-response
title: Validation arm must accept time.Time, not just string (yaml auto-decodes datetimes)
finding: 'yaml.v3 auto-decodes an unquoted RFC3339 scalar in frontmatter to Go time.Time, not string (verified: `due: 2026-07-13T12:30:00Z` -> time.Time; only survives as string if quoted). markdown/parser.go:85 returns the raw map with time.Time intact (no normalization). A datetime validation arm shaped like the date arm (validation.go:235, `s, ok := val.(string)`) will reject hand-edited unquoted datetimes with ''Must be a datetime string''. The existing date type shares this latent bug but rarely bites. Fix: the datetime arm must accept BOTH string (parse via ParseDateValue) and time.Time (already an instant), normalizing to a canonical UTC RFC3339 instant. Note format:date-time in openapi is cosmetic; the yaml parser decides the Go type.'
severity: critical
resolution: 'Confirmed by running yaml.v3: unquoted RFC3339 and bare dates both decode to time.Time; markdown/parser.go:85 returns them un-normalized. Datetime validation arm will accept a type-switch: case string -> parse via ParseDateValue; case time.Time -> already a valid instant, accept. Empty/absent -> OK unless required. Mirrors the requirement, fixes the hand-edit-rejection bug the date arm latently has.'
status: addressed
---
