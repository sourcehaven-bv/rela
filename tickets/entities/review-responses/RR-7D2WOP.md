---
id: RR-7D2WOP
type: review-response
title: appIDRegex duplicated in apps_handler.go and validate.go (drift risk)
finding: 'Identical appIDRegex `^[a-z0-9_-]{1,64}$` defined twice (apps_handler.go:31, validate.go:923); the handler comment even says ''Must match the regex used in validateApps''. Duplication begging for drift. FIX: export one and reference it, or add a grep/test pinning them equal.'
severity: minor
resolution: Removed the duplicate appIDRegex from apps_handler.go. Exported dataentryconfig.ValidAppID(id) (backed by the single appIDRegex in validate.go) and the handler now calls it — one source of truth, no drift.
status: addressed
---
