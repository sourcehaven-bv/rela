---
id: RR-H25H0
type: review-response
title: Legacy .rela/key read-with-warn is worse than today for planted-key attacks and slog.Warn is invisible in MCP
finding: 'Proposed loader: env → us.Path(key) → legacy .rela/key with slog.Warn. Attacker who writes to repo tree (Dropbox collab, malicious PR) plants .rela/key; loader uses it silently for most users because slog.Warn goes to stderr, which MCP-over-stdio discards and busy CLI users ignore. Also: ''remove in follow-up'' is not scheduled.'
severity: critical
resolution: 'User decision: no fallback. Drop the legacy .rela/key read tier entirely in this PR. Identity precedence becomes: $RELA_KEY_FILE → us.Path(key) only. Users with an existing .rela/key must either move it manually or re-run rela keys init. Release notes must document the hard break. Simpler, safer, no deprecation schedule to track.'
status: addressed
---
