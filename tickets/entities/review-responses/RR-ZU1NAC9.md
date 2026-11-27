---
id: RR-ZU1NAC9
type: review-response
title: Fixture note on authorization deps is stale
finding: fixture.go says each authorization dep is required by entitymanager.New or taken from TransitionWiring; FieldGate was required yet drifted because AllowAllFieldGate satisfies the requirement.
severity: minor
reason: The note is an in-function comment and editing it trips the comment lint. BuildFieldWriteGate and its doc now close the drift it describes.
status: deferred
---
