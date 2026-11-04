---
id: RR-KT4LFHB
type: review-response
title: Quiet cause in a working state splits the fold
finding: decisionSeq is set whenever the reader has any decision (useSessionDecisions.ts:203), also in a working state with an owner Stop. A quiet progress message that moved the session is then kept with the buttons and revealed for the rest of the mount, so every move still splits the fold.
severity: significant
resolution: 'Plan: decisionSeq is undefined when the current state is working and the cause type is quiet; the decisions show in the session header (placed.fallback). Waiting states keep the TKT-A0RTC52 placement. AC 4a with tests in useSessionDecisions and ChannelsPanel.'
status: addressed
---
