---
id: RR-WNA8BQ
type: review-response
title: 'Review minors: depth-cap semantics undocumented, principal-dependent 422s, latent nil deref, duplicate hierarchy entries, default_depth>max_depth, misc nits'
finding: 'Bundle of the review''s minor findings: (1) MaxDepth''s doc said ''caps the server-side walk'' while the implementation caps per-response relative to the drill root; (2) on_cycle/multi_parent error verdicts are principal-dependent (correct, but undocumented — an admin cannot reproduce a report against hidden entities); (3) linkGanttParents deref 20 lines from its guard; (4) duplicate hierarchy entries passed validation and were harmless only by coincidence; (5) default_depth > max_depth loaded clean but was silently unachievable; (6) frontend nits — no-op setUTCHours, undocumented local-time todayDay, expansion state surviving drills unboundedly.'
severity: minor
resolution: '(1) MaxDepth field doc rewritten (payload cap, flame-graph semantic, drilling re-roots) + docs guide table row + TestGantt_DrillDepthIsRelative pins it; (2) docs guide paragraph added on principal-dependent error policies; (3) nil guard moved adjacent to the deref; (4) duplicate hierarchy entries are now a load error with a message naming both indices, tested; (5) effective default_depth vs max_depth validated (including against the implicit default), tested; (6) no-op line dropped with a comment, todayDay''s deliberate local-calendar choice documented at the site, expanded set cleared on every drill (context change + unbounded growth). Not taken: ticksFor guard rework (backstop only, nit) — an axis-outlier warning is better solved by flagging degenerate spans at the data level if it ever bites.'
status: addressed
---
