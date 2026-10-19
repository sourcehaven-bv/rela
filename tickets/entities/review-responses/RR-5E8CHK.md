---
id: RR-5E8CHK
type: review-response
title: A transient-classified error that is really per-entity and permanent stops the whole round on every retry
finding: 'internal/realtime/system.go post and trigger.go postOne default branch. Only validateProps, the when: check, a type-shape conflict, ErrTooManyChannels and ErrTooLarge are wrapped in permanentError. Append can also return ErrStaleAnchor (the channel belongs to an earlier entity with this id), ErrInvalidKey and ErrInvalidMessage. These fail the same way on every try, but they are treated as an outage. Each attempt returns stop=true, so every retry of such an entity ends the round for all rules, for the full five-minute window (about 12 attempts with the doubling delay). After an entity id is reused, its post is delayed for five minutes and then reported as ''keeps failing''. A bulk re-create cuts trigger throughput to roughly one entity per round. Fix: classify ErrStaleAnchor, ErrInvalidKey, ErrInvalidMessage and ErrTypeShapeConflict as permanent, and stop the round only for errors that are about the store as a whole rather than this key.'
severity: minor
resolution: ErrStaleAnchor, ErrInvalidKey, ErrInvalidMessage and ErrTypeShapeConflict are now permanent, next to ErrTooManyChannels and ErrTooLarge. Only the remaining errors, which are about the store as a whole, stop the round; TestTriggers_PerKeyErrorIsPermanent pins it.
status: addressed
---
