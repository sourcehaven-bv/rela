---
id: RR-IISNHQZ
type: review-response
title: The suspect hash brings the bug back when a state flips twice
finding: A served hash equal to the suspect was dropped. The same transient state seen twice (claim and release twice with a fetch in between each time) then sent no frame.
severity: significant
resolution: 'Replaced suspect with a streak counter: records stop only after liveServedStreak frames in a row with no matching replay or served baseline. TestLiveSub_ServedRecords and TestLiveSub_ServedStreakStopsLoop pin both sides.'
status: addressed
---
