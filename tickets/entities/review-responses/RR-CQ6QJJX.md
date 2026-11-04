---
id: RR-CQ6QJJX
type: review-response
title: No permission signal on the wire for row actions
finding: _self is an address, rows carry no _actions, and relation affordances are deferred (internal/dataentry/CLAUDE.md). The menu would show to everyone and fail with 403. Needs per-row _actions.delete and a section unlink verdict via translateVerb, with contract and budget tests.
severity: significant
reason: Moved with the scope split (RR-CPB5PXW) to TKT-48ZHQZ3, which carries this constraint in its description. This ticket ships no row delete, unlink or Undo action, so the defect cannot occur in this change; it must be solved before TKT-48ZHQZ3 ships.
status: deferred
---
