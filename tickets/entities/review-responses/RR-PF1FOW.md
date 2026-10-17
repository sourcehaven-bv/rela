---
id: RR-PF1FOW
type: review-response
title: Live isolation test cannot tell a topic round from a full round
finding: Frames depend on replay hashes, so a full replay sends the same frames; setting dirty on chanWake keeps the suite green.
severity: significant
resolution: The test counts replays per URL with a second readable channel on the anchor; a post replays only that head and the anchor list. It fails when chanWake forces a full round.
status: addressed
---
