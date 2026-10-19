---
id: RR-5XRP91
type: review-response
title: One auto act can make up to 200 writes but costs one attempt from the 64-per-round budget
finding: "internal/realtime/system.go:72 transition moves every listed channel in a from state. On a cardinality: many type that is up to channels.MaxChannelsPerType (200) SetLifecycle calls. postOne counts the act as one attempt toward MaxSystemPostsPerRound, so one round can make about 64*200 writes and local marks. That defeats the cap's purpose of bounding system writes per round (D11).\tA bulk status change moves 64 tickets to done, and each ticket has 150 open threads under a threads archive/close auto.when. One round makes 9600 lifecycle writes and audit rows in a single 500ms tick.\tCharge the budget per channel moved: return the count and add it to attempts, or cap the moves per act and requeue the rest (a second call already resumes, as the doc comment says). Alternatively document that the cap counts entities, not writes."
severity: minor
resolution: transition takes a limit and reports tried/moved/more; each channel tried costs one attempt (an act that finds none costs one). An act with more channels than the remaining budget moves what fits and goes back to the head of its queue. TestAuto_MovesCountAgainstTheCap; docs say moves count against the 64.
status: addressed
---
