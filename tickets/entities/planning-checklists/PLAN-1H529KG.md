---
id: PLAN-1H529KG
type: planning-checklist
title: 'Planning: Manual start condition, mail button and hiding empty dashboard cards'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood (three gaps found while configuring the harbourmaster portal: no condition on manual flow starts, no button in mail.render, no way to hide an empty dashboard card)
- [x] Scope defined (what's in/out documented below) (in: when: on on: manual, mail.render button, dashboard card hide_empty, docs. Out: conditions on other surfaces, several buttons per mail, hiding cards on other conditions)
- [x] Acceptance criteria documented with specific test scenarios (see the ticket body)

**Scope:**
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: three small options)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: config keys in existing code)
- [x] Checked codebase for similar patterns or reusable code (the create start's when: and OnCreate.anyHolds; mailrender's safeHref for links; hide_empty on view sections (TKT-JT101B) for the name)
- [x] Looked for reference implementations in other projects (the bulletproof button shape that email frameworks such as MJML compile to: a link in a colored table cell)
- [x] Reviewed relevant rela concepts for prior art (flows, lua-scripting, data-entry-ui)

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented (flowcfg accepts when: on manual; flowengine.ManualStarts evaluates the guards over the raw subject; FlowControl.Start and Startable use the permissions of the starts whose condition holds, and refuse a start with errFlowCondition (conflict). mailrender.Message gains Button, rendered below the intro and vetted by safeHref; the Lua binding parses button = {label, url}. DashboardCard gains HideEmpty; the SPA filters out a loaded card with no rows)
- [x] Approach builds on existing patterns (not reinventing) (guard compile and evaluation are the flow engine's own; button URLs reuse safeHref)
- [x] Alternatives considered (document why rejected) (a server-side filter of empty cards would need the dashboard endpoint to compute every card value; the SPA already loads them. Dropping an unsafe button URL was rejected: a dead button is a script bug, so it raises)
- [x] Dependencies identified (packages, APIs, types) (none new)

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] Input sources identified (user input, config, external APIs) (flows.yaml and data-entry.yaml are operator config; mail.render arguments come from a Lua script and are untrusted)
- [x] Input validation approach defined (allowlist preferred over blocklist) (when: goes through the guard allowlist; the button URL through the http/https/root-relative allowlist; the label is escaped by html/template)
- [x] Security-sensitive operations identified (file access, auth, crypto) (manual start authorization: the permission check stays first, so a caller without it learns nothing about the condition)
- [x] Error handling doesn't leak sensitive information (a guard evaluation error is logged at debug only, since it can quote a property value)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion (TestFlowControl_ManualStartWhen, TestParse_Problems cases, TestRender_Button, TestMailRender_ButtonErrors, TestValidateDashboardCardHideEmpty, DashboardView hide_empty tests)
- [x] Edge cases identified and documented (several manual starts with different conditions; relative button URL without a base URL; failed card with hide_empty)
- [x] Negative test cases defined (invalid input, error conditions) (manual start condition on a hidden property, every on manual, javascript: button URL, missing label, hide_empty on a line card)
- [x] Integration test approach defined (not just unit tests) (FlowControl tests run on a real project on disk; the Lua test pins the binding to the Go renderer byte for byte; the compat test scores the button against the Can I Email data)

**Test Scenarios:**
<!-- Map each acceptance criterion to how it will be tested -->

**Edge Cases:**
<!-- List specific edge cases and expected behavior. Consider:
- Empty/null/missing values
- Boundary values (0, -1, MAX_INT)
- Special characters, unicode, null bytes
- Concurrent access
- Resource exhaustion
-->

**Negative Tests:**
<!-- What should fail? How should it fail? -->

## Risk Assessment

- [x] Technical risks assessed with mitigations (button CSS breaking in Outlook: padding on the cell only, compat test covers it)
- [x] Security risks assessed (see Security Considerations) (covered above)
- [x] Effort estimated (xs/s/m/l/xl) (s)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor) (GUIDE-flows, GUIDE-mail, GUIDE-lua-scripting, GUIDE-data-entry)
- [x] Docs-checklist will be created when entering implementation (created in review)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes (via GUIDE-data-entry; also GUIDE-flows, GUIDE-mail and GUIDE-lua-scripting)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: no new convention)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: there are user-facing docs)
-->

## Design Review

- [x] Run `/design-review` before starting implementation (self-review of the design against the guard, mail and ACL rules; no findings)
- [x] All critical/significant findings addressed in plan (none)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
