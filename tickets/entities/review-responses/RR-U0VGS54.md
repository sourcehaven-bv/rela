---
id: RR-U0VGS54
type: review-response
title: Drawer start does not wait for the first message page or the reveal jump
finding: OpenChannel runStart gates only on head, actionsGeneration and revealDone; revealDone is set right after jumpTo returns, which only emits jump when the seq is not held. Other on an older question always takes the onAnswer fallback and the documented gate (head and first page held, reveal run) is not what the code does.
severity: significant
resolution: OpenChannel runStart waits for the first page when last_seq > 0 and for any running jump (a jumps counter around onJump). OpenChannel.start.test.ts holds the older page back and checks the reply survives the swap and posts with reply 41.
status: addressed
---
