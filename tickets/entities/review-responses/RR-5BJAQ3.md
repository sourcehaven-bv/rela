---
id: RR-5BJAQ3
type: review-response
title: A click on a threaded question does not address the asker
finding: 'Q3 (plan lines 786-789) says reply_to "addresses the asker (an inbox reply row for the agent)". Threads are one level deep (internal/channels/channels.go:92-94), and the plan sets reply_to to the thread root when the question is itself a reply (AC 9, step 8). A reply row goes to the author of the parent (internal/realtime/inbox.go:148-152), so the answer addresses whoever wrote the thread root, not the agent that asked. Also, a question in a thread whose root was removed renders live buttons whose click fails with ErrInvalidReply (400), which AC 9 routes into the composer as if it were a form error. Fix: offer choice buttons only on top-level questions in this slice (a threaded question shows its options as a plain list), or state the addressing limitation explicitly and hide buttons when the root is removed or not held. Correct the Q3 text either way.'
severity: significant
resolution: Choice buttons are offered only on top-level questions and the click replies to the question itself, so the reply row addresses the asker. A threaded question shows its options as a plain list, and Q3 now states why.
status: addressed
---
