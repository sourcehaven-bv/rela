---
id: RR-MNX2LB9
type: review-response
title: pingSeq hides a token that fails to open
finding: A ping token that fails to open made pingSeq return 0 forever, so drain timed out with a message that blamed the loop.
severity: minor
resolution: pingSeq now fails the test naming the token.
status: addressed
---
