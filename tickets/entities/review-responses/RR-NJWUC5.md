---
id: RR-NJWUC5
type: review-response
title: Unbounded timeline width on an outlier date
finding: A 2062 typo made a 525,000px timeline with ~1,900 gradient layers per row; tick guard silently stopped.
severity: significant
resolution: pxPerDay capped at MAX_TIMELINE_PX (250,000) with a 'compressed' flag; ticks thinned to >=56px apart; guard raised to 20,000 with comment; unit test.
status: addressed
---
