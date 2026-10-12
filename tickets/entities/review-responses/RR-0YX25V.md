---
id: RR-0YX25V
type: review-response
title: Replay chain re-lists the production response wrappers
finding: 'Architecture review: liveReplayChain applied attachWorld and attachACLRequest by hand, kept in step with NewRouter only by a comment. A new body-changing wrapper added to NewRouter alone would make replays drift from GETs with no failing test.'
severity: significant
resolution: responseGates applies both wrappers; NewRouter and liveReplayChain each call it, so the list exists once. The NewRouter comment says to add body-changing wrappers there.
status: addressed
---
