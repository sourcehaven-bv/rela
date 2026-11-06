---
id: RR-PA1O5WN
type: review-response
title: Incoming move judges every face of an implicit-tail sibling source
finding: For an implicit-tail sibling the gate checks every face of its source including faces the caller cannot read; a refusal there fails the move with a 403.
severity: minor
reason: This is the rule checkRelationMetaWrite and the data-entry handler already apply to every relation meta write (BUG-O8QP7GX); it discloses at most that a grant refuses the order key and no content or existence. Narrowing it belongs with BUG-T0MFVJG.
status: wont-fix
---
