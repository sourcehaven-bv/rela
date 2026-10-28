---
id: RR-YXRV9M
type: review-response
title: AC 11 does not match the selection rule
finding: AC 11 says a message, a mention and a session move on a channel Alice cannot read all send no frame "although the replay runs". Under the plan's selection rule, a plain message in a hidden non-waiting channel outside her window does not replay at all. Only the mention (an addressed target) and the session move (a waiting type) replay. State per case whether a replay is expected, and give the test a replay counter (for example a wrapper around the replay handler) so the statement "the replay runs" is checked, not just assumed.
severity: minor
resolution: Rewrote AC 14 per case with the expected replay (none for a plain hidden message outside the window, replay without frame for a hidden mention and a hidden session move) and a replay counter around the replay handler.
status: addressed
---
