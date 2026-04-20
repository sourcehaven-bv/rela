---
id: RR-HQGU7
type: review-response
title: state.FSKV writes 0o644; security plan says 0o600 for user-state
finding: internal/state/state.go:60 uses 0o644 for all Puts. If user-state reuses FSKV, user-defaults.yaml etc. are 0o644 in user home dir. Inconsistent with plan's security text.
severity: minor
resolution: userstate.Service uses 0o600 files, 0o700 dirs uniformly. Don't reuse state.FSKV's permission defaults — have userstate ship its own FS-backend that enforces stricter perms. Still implements the state.KV interface so consumer code is unchanged.
status: addressed
---
