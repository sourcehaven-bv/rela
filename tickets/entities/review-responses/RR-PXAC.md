---
id: RR-PXAC
type: review-response
title: No acceptance criteria, so 'done' is undefined for a change whose main risk is silent breakage
finding: 'The ticket has a Proposal, Scope, Compatibility and Non-goals but no acceptance criteria. For
  most refactors that is tolerable. Here it is not, because the identified failure mode is SILENT: removing
  writes also removes rela.bypass_acl with no compile error.


  At minimum the ticket should name the tests that must stay green and the ones that must be added:


  - TestElevatedRender_ReadsHiddenEntityAndAudits must still pass (the existing canary; the ticket already
  says "do not weaken it" but does not make it a criterion).

  - A new test asserting rela.create_entity is ABSENT in a document runtime — the positive statement of
  the change.

  - Equivalent coverage for all three entry points, since they share runDocumentScript.

  - Something pinning that export_render still works, whatever is decided for it.


  Without these, "did we finish?" is answerable only by reading the diff.'
severity: minor
status: open
---
