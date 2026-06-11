---
id: RR-W6LX
type: review-response
title: Tests assert row count and badge count but not message content fidelity
finding: The new 'renders a Duplicates card...' test asserts 2 rows and count=2, but doesn't check that the message column actually carries the duplicate text. Same for the clickable state - Duplicates rows have a real entityId so they should be .clickable, but the test doesn't assert it. A regression that drops the message column or strips the clickable class would pass silently. Add message-substring check and .clickable class assertion (mirroring the inverse assertion in the ID Gaps test).
severity: significant
resolution: 'The Duplicates test now asserts: (1) the row text contains the duplicateMessage variable that was fed in, and (2) rows[0].classes() contains ''clickable'' (Duplicates rows have a real entityId). Mirrors the inverse assertion already present in the ID Gaps test.'
status: addressed
---
