---
id: RR-OKB9TC
type: review-response
title: e2e fixture under-specified and skips the message-row buttons
finding: RoomOptions.decisions needs the `contained` message type, which today is emitted only under opts.session (realtime-project.ts:68-80), and it defines a second session block. Say whether decisions replaces session (mutually exclusive) and whether it needs a third server fixture or a runtime writeRoomRealtime rewrite. The page object lists expectDecisionOnMessage, but the spec steps never assert the buttons on the Contained message (the cause of the review state). Add that assertion, and assert that the member's row shows none. That is the only end-to-end proof of decisionCause against real notices.
severity: minor
resolution: RoomOptions.decisions is mutually exclusive with session (throws if both), uses its own decisionTest server fixture, and the spec asserts the buttons on the deciding Contained message for the lead and their absence for the member.
status: addressed
---
