---
id: RR-OPBDAY
type: review-response
title: A nameless automation produces blank-filled diagnostics across engine runner and analyze
finding: Several operator-facing messages render automation "" or automations. when the name is empty
severity: minor
reason: Cosmetic diagnostics only (automation "" in three log/error strings and automations. in analyze). Not load-bearing and none is on the audit record itself. The real remedy is making an empty automation name impossible at load time - a metamodel-loader change that belongs with RR-2FY0O8's validation work rather than in an attribution ticket. Deferring so the fix lands once in the right place instead of papering over each string.
status: deferred
---

Cosmetic fallout of the same empty-name case, all low priority but all in the
diagnostics an operator reads *after* a nameless automation has misbehaved:

- `runner.go:293` — `automation "": no ScriptRunner configured`
- `engine.go:158`, `:99-126` — `automation "": when clause ...` /
`automation "": condition requires ...`
- `schema/analyze.go:171-180` — renders the display string `"automations."`

None is load-bearing. Noted rather than fixed here: the real remedy is making an
empty automation name impossible at load time (see the load-time validation
finding), after which these strings can never be blank.
