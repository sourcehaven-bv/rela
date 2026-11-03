---
id: RR-8TGOK9
type: review-response
title: Stepper precedence and the collapsed layout are undefined for combined states
finding: 'The plan gives final, stranded and paused one by one but not combined. A closed channel whose session is final would read "Paused while the channel is closed" next to "Ended", which is wrong. Stranded plus closed is also unspecified. The narrow layout shows "Session · ‹current label›", but a stranded session has no current. Plan change: state the precedence final > stranded > paused for the caption. In the collapsed layout, show "Stranded · ‹stored name›" for a stranded session. Add stories FinalClosed and NarrowStranded to AC 9.'
severity: minor
resolution: 'Plan updated: precedence stranded > final > paused > normal for caption and collapsed summary ("Stranded · ‹name›"); AC 10 adds FinalClosed, StrandedClosed and NarrowStranded stories.'
status: addressed
---
