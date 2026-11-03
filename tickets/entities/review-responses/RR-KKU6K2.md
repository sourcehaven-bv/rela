---
id: RR-KKU6K2
type: review-response
title: Timer-only cycles are allowed and spin forever at one move per minute per channel
finding: 'The loop check refuses only cycles made of from: session timers. A from: state self-loop (explicitly accepted, AC 3) or a mixed cycle (A -from: session-> B -from: state 1m-> A) re-enters every period forever with no message involved. Each move appends a notice and an audit record, so one channel produces 1440 notices and 1440 audit rows per day, for every open channel of the type, indefinitely. Repeating reminders are slice 4 (in:/every:) and have no use case here. Plan change: refuse any cycle made only of after: edges in this slice (one DFS over the timer graph, the same pass as the from: session check), drop AC 3, and add a row ''a from: state self-loop'' plus ''a mixed timer cycle'' to TestParse_SessionTimersInvalid. Revisit when slice 4 adds a bounded repeat.'
severity: significant
resolution: 'Plan updated: any cycle made only of after: edges is refused, self-loops included; old AC 3 replaced by cycles through on: loading; invalid rows for since: state/session self-loops and a mixed cycle.'
status: addressed
---
