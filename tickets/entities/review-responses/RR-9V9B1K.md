---
id: RR-9V9B1K
type: review-response
title: RELA_UNCONFINED_COMMANDS edge case needs no special handling; plan overstates it
finding: 'The plan''s Edge Cases list says the deliberate unconfined opt-out (RELA_UNCONFINED_COMMANDS=1) ''must NOT fire the new WARN'', implying explicit suppression logic. It needs none: cmdexec.New''s three-way switch (cmdexec.go:141-143) takes the sandboxOptOut branch FIRST and leaves sandboxErr nil, so a `SandboxErr() != nil` condition already excludes the opt-out by construction. Keep the regression test (it pins the interaction), but drop the implication that special-casing is required — writing suppression logic for a case the type system already excludes would be dead code.'
severity: minor
resolution: 'Plan''s edge-case entry rewritten: states that no suppression logic is needed because cmdexec.New takes the sandboxOptOut branch first and leaves sandboxErr nil, so a `SandboxErr() != nil` condition excludes the opt-out by construction. The test is retained to pin the interaction, but the plan no longer implies special-case code.'
status: addressed
---
