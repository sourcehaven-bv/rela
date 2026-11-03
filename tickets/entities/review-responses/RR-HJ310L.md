---
id: RR-HJ310L
type: review-response
title: Round cap drops the same tail every round
finding: 'Security and code review: collect sorts pairs deterministically. Pairs past the cap were cut every round. After the first fallback cleared their hash they were never replayed or framed again. The test masked this by running a baseline round second.'
severity: significant
resolution: The cap start rotates each over-cap round. fallback frames and keeps the baseline. TestLiveHub_RoundCapFallsBack now runs two changed rounds and checks the cut one is replayed and framed.
status: addressed
---
