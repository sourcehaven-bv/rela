---
id: RR-G5TJKC
type: review-response
title: Docs say switching composer type drops the form decision; the code drops it only when a post of another type is sent
finding: 'GUIDE-realtime.md:1656-1657 and the generated docs/realtime.md:1650-1651 say "Choosing another type in the composer drops it as well". Deviation 6 and useSessionDecisions.ts sendComposer clear formDecision only when a send of another type happens. Switching away and back before sending keeps the decision and still sends expect_generation. Correct the docs to describe the actual behavior: the decision ends when you send a message of another type, when the session moves, or after the send. Deviation 6 is safe server-side: a stale generation is refused with session_moved, and the watch clears the decision on a head move outside a send.'
severity: significant
resolution: The composer now emits select when the type of a new message changes, and OpenChannel drops the form decision on a switch to another type, so the docs are now correct; they also say that switching back before Send does not restore it, and a ChannelsPanel test pins the wiring.
status: addressed
---
