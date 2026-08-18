---
id: RR-LWD8N3
type: review-response
title: An elevated document is an unbounded read oracle unless the script is trusted not to echo what it reads — no mechanism enforces that
finding: |-
    Once a document renders under elevation, the ACL no longer bounds its OUTPUT — only the script's own discipline does. rela.bypass_acl(function(admin) ... end) can read any entity and print it. A holder of the document's `permission:` therefore effectively holds read access to everything the script chooses to emit, with no per-row gate anywhere downstream.

    That makes `permission: report:sales` a much broader grant than it looks: it is not 'may view this report', it is 'may read whatever this script reads'. An operator reading data-entry.yaml sees a report name; the actual privilege is defined in a Lua file somewhere under scripts/. The two drift independently, and nothing fails when they do.

    This is the same shape as RR-37AYC0 on this project ('entity/list command payloads bypass the read gate, so a command:* grant is a read-everything oracle — contradicting the docs shipped in this ticket'), which was raised and addressed. The elevated-document proposal reintroduces it in a new place, and the ticket does not mention it.

    The cascade precedent does not transfer. A cascade script's elevated reads feed WRITES whose results are themselves ACL-gated on read-out, so elevation there does not directly become a read channel to a human. A document render's entire purpose is to put bytes in front of a person, so elevation there IS the read channel. The ticket's 'documents render, they do not mutate' non-goal is stated as if it made the feature safer; for read confidentiality it is precisely what makes it more dangerous.

    At minimum the ticket should state that an elevated document's script is TRUSTED CODE — equivalent in blast radius to granting its permission-holders the union of what the script reads — and say where that trust is documented for the operator. Consider whether the audit row (acl-bypass-read) is sufficient after-the-fact accounting, given it records the read but not what reached the page.
resolution: |
  Accepted as DOCUMENTED-NOT-ENFORCED (operator decision).
  
  Option A cannot enforce that an elevated script emits only aggregates: bypass_acl
  hands it a raw reader and nothing stops it printing what it reads. That is
  inherent to the chosen approach, not an oversight.
  
  Mitigation is documentation, and the docs must say the uncomfortable thing
  plainly: an elevated document's script is TRUSTED CODE, and its `permission:`
  grants read access to everything the script reads, not merely to the report. This
  goes in docs/acl-security.md and in the DocumentConfig.Permission godoc, which
  also gains the conditional rationale (gated reads => guards against a report
  claiming a scope it did not compute; elevated reads => IS the confidentiality
  boundary).
  
  Option C in RES-XZBZXB (an aggregation primitive that structurally cannot return
  rows) is the answer that would make this property enforceable rather than
  promised. Kept on the roadmap for if/when aggregate-over-hidden-rows becomes a
  recurring pattern rather than one report.
  
  Related prior art: RR-37AYC0 (command payloads as a read oracle) is the same
  shape and was addressed by documenting the boundary rather than narrowing it.
severity: significant
status: addressed
---
