---
id: RR-RKPV1V
type: review-response
title: Timer plus trigger outputs allow unattended graph-writing loops the loop rule assumed impossible
finding: 'GUIDE-realtime.md:657 justifies refusing timer-only loops because they move the session ''without anyone posting'', and allows any loop through a message transition. With run_as, a timer output can change a property that a trigger watches, and the trigger''s post fires a no-roles transition back. That loop now runs forever with no person, writing as run_as every cycle. Only the period and the trigger cooldown bound it. timerLoops (realtimecfg/session.go:340) treats every message transition as a break. Fix: at least document this in the Loops and Automations paragraphs. Better: in timerLoops, count job transitions (t.job) as non-breaking edges, or warn when a job output sets a property a trigger into the same channel type reads.'
severity: minor
resolution: A loop of timers and trigger-fired transitions with outputs on it is refused at load (without outputs nothing writes); TestJobLoops has 4 cases; guide updated.
status: addressed
---
