---
id: RR-AAV65L
type: review-response
title: Some acceptance criteria are not testable as written
finding: 'AC 9 ("dots animate only under prefers-reduced-motion: no-preference") and AC 10 ("a change is announced once") cannot be checked in jsdom. They should be rephrased as structural assertions: the animation is declared only inside the no-preference media block (story plus a CSS assertion), and the activity line renders inside the existing role=status element with no second live region. AC 5 "never cached across principals" needs a concrete test: two principals subscribed to the same busy URL in one live round, one denied the channel read, get different hashes. AC 12 has no test and should say "reviewed".'
severity: minor
resolution: AC 9 and AC 10 are now structural checks (aria-hidden, no live region on the indicator, the reduce media block in the source, the line inside the one role=status element), AC 5 has a two-principal HTTP test and a live per-principal hash test, and AC 12 says "reviewed".
status: addressed
---
