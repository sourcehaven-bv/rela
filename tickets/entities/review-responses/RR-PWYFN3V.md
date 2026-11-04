---
id: RR-PWYFN3V
type: review-response
title: Pre-check and write can race with a peer conversion
finding: The handler checks types from a read; an entity conversion between that read and the write could change a peer type. The plan must not rely on the handler check alone.
severity: nit
resolution: The manager re-validates in prepareRelationFrom with no tolerate option; dataentry maps that InvalidRelationError to the same 422 code.
status: addressed
---
