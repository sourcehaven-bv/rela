---
id: RR-LF36QW
type: review-response
title: The reference button dirtied the document before the user committed
finding: _promptForRef types '@' into the document to open the menu. Pressing the button and then changing
  your mind left the character behind AND marked the document edited for the rest of the session, so the
  next .value read handed the app a fully reserialized body it never asked for. A probe showed the cursor
  can be anywhere the user left it, including inside a table cell.
severity: nit
resolution: 'SUPERSEDED by code review. The first fix withdrew the trigger text but left the document
  marked edited, on a stated rationale that turned out to be unverified, and it consumed its own record
  before checking it could act — so a readonly editor stranded the character permanently, reintroducing
  this same defect through a different door. See RR-7QE2I8 and RR-4GELDN for the real fixes. What holds
  now: the span is consumed only on a path that resolves it, and the edited-state is restored when the
  serialization matches what the editor produced immediately before the trigger was typed.'
status: addressed
---
