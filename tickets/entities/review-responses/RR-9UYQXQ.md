---
id: RR-9UYQXQ
type: review-response
title: Guard-failure log text is wrong after fall-through and does not name the entry
finding: 'logGuardFailure (internal/realtime/session.go:470-477) logs "the session did not move", but under the plan a failing entry 1 falls through and entry 2 may move the session. The log also names only state and message type, so with several entries the operator cannot tell which guard failed. Fix: log "this entry was skipped", add `to` and the entry key (t.Key()), and keep WarnGuardOnce per entry.'
severity: minor
resolution: '`logGuardFailure` now says the entry was skipped and logs `to` and the entry key, still once per entry.'
status: addressed
---
