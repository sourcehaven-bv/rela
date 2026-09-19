---
id: RR-SZJZX9
type: review-response
title: Three spellings of direction with two hand-written mappings, each defaulting to outgoing
finding: 'Code review, leverage item. The same two-valued concept now exists as metamodel''s strings (RelationDirectionOutgoing/Incoming), validation.Direction (an int enum), and store.Direction, with two hand-written mappings between them (validation.go''s dir selection, validationgraph''s directionOf). Each mapping has a `default: outgoing` fallback — the silent-wrong-direction failure this ticket exists to prevent. A third value added later would take the default silently in both.'
severity: minor
resolution: Not changed. The load-time rejection of an invalid direction (validateConstraintDirection, covered by TestValidateValidationRelations_NewKeys) is what actually prevents the failure mode; the fallbacks are defence behind a closed door.
reason: 'Deferred rather than fixed: the two mappings are each three lines and both are unreachable for an invalid value, because the loader rejects any direction that is not one of the two words before a Program ever runs. Making them exhaustive switches with a panic default would add a panic path to a validated input for a hazard that is already closed at load. The real generalization — one direction vocabulary shared by metamodel, validation and store — is a refactor across three packages and two arch-lint boundaries, which does not belong in a ticket about relation gates. Recorded here so the next person adding a direction value sees the three sites named.'
status: deferred
---
