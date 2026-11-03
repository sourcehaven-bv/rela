---
id: RR-RB29WI
type: review-response
title: Deadline-reset refusal misses a two-state handoff cycle
finding: 'internal/realtimecfg/waiting.go:204 (waitChecker.deadlineReset) only refuses a direct re-entry (t.To == st.Name). A holder of a waiting role can still push a since: state deadline back forever through a cycle: in contained (since: state timer, waiting_for [lead]) the lead posts handoff to: themself -> handed-over (waiting_for [lead], now themself) -> accept-handoff -> contained, and entering contained again restarts its since: state timers and reminders. The config loads clean, and docs/realtime.md (rules list, ''or hand off to a separate state, as the example does'') presents a separate hand-over state as a safe alternative to since: session, which it is not when the waiting roles can return to the state. The shipped example is safe only because both states use since: session. Fix: extend the check to refuse, in a state with since: state timers or active reminders, any path of transitions fireable by the waiting roles (the same fireableBy rule, through states whose own waiting roles can fire the next hop) that leads back into the state. If that is out of scope, reword the docs so the only stated remedy is since: session, and record the cycle as a known residual next to the non-waiting-role re-entry residual.'
severity: minor
resolution: 'deadlineReset refuses any chain of waiting-role moves back into a state that restarts since: state timers or reminders; example adjusted; guide names since: session as the remedy; two-state cycles tested'
status: addressed
---
