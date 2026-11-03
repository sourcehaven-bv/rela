---
id: RR-OJH077
type: review-response
title: Reply-quote reveal does not work for a target that is not held
finding: 'The edge case ''jump loads around it as today, and the reveal then applies to the loaded page'' is false. onJump (useChannelPaging.ts:42-47) loads older pages until the seq is held and then returns. Nothing calls back into RlChannel, which in plan step 6 adds a seq to revealed only when it is already held. A hidden target loaded this way stays hidden, and nothing is scrolled to or focused. Today the same path also does not scroll to a visible target. Change the plan: jumpTo adds the seq to revealed (or to a pending jump) before it emits jump, and a watch on messages scrolls to and focuses that row once it arrives. Add a ChannelsPanel case with the hidden target on an older page.'
severity: significant
resolution: useChannelPaging takes an onHeld callback that onJump calls once the target is held, and OpenChannel.vue passes it to a new exposed RlChannel.reveal, which shows, scrolls to and focuses the target (step 7, files listed). Added the ChannelsPanel case with the hidden target on an older page and fixed the edge case text.
status: addressed
---
