---
id: RR-FG4ME1
type: review-response
title: The Security section's "no new channel" claim understates how far selection now reaches
finding: Two kinds of move now add replays to a round. (1) A move on a channel the subscriber may not read, on a card it can see, selects its own board (AC 7). (2) A move on a card hidden from bob selects alice's board. Both lengthen the round and can move where the cap cuts. A channel subscription that is cut still gets a fallback frame (live.go:912-914), so another principal's frame can now depend on a session move that principal cannot see. This is the same class as the inbox's type-wide selection (live_inbox.go:268-272), which live_inbox.go:40-44 accepts, but the plan should say that it widens the existing channel rather than claim that nothing is added. Optionally, sort exempt pairs (inbox, busy) after the other pairs before the cap cut in replayRound (live.go:750-757). A pair chosen by a hidden change could then never push a frame-receiving pair into fallback. The cost is weaker rotation fairness for the exempt pairs, which the rotating cursor and pending then carry.
severity: minor
resolution: 'Adopted the wording: the Security Timing paragraph now says the change widens the existing timing channel, names both new selection paths and the fallback frame of a cut pair, and accepts it like the inbox. The optional sort of exempt pairs is recorded as rejected alternative 10 because it starves those pairs under load.'
status: addressed
---
