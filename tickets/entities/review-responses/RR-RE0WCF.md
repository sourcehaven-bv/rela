---
id: RR-RE0WCF
type: review-response
title: replayPrincipalFrom falls back silently
finding: 'Architecture review: without the stash middleware it returned the resolved principal, so a moved middleware would make replays resolve twice with nothing reported.'
severity: minor
resolution: It returns (Principal, bool). A stream without the stash gets no live views and logs an error; a PUT without it gets 404. Pinned by TestSSE_NoStashedPrincipalNoLiveViews.
status: addressed
---
