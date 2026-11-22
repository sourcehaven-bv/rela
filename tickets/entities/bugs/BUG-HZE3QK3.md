---
id: BUG-HZE3QK3
type: bug
title: Flow step that no branch accepts stays put without a message
description: Completing a step when no change branch's guard holds saves the fields and leaves the run waiting; the form says nothing.
priority: medium
effort: s
why1: The step answer carries no problem when no change branch's guard holds; the run card then shows the plain success toast and reloads.
why2: The engine returns the state with no error on no move, the same as when the caller may take no branch at all, so the answer cannot tell the two apart.
why3: The problem values were added for failures after the values were saved (guard error, moved, settling); a guard that is simply false was treated as the normal waiting case.
why4: The tests pinned the run state after such a step but not what the person is told.
why5: No test checked the message a person sees for each outcome of completing a step.
prevention: TestFlowControl_CompleteStep pins the problem for each outcome, including blocked; the FlowRunCard test pins the warning.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

In the harbourmaster portal, completing the "Company details" step without the
terms checkbox (guard `entity.terms_accepted == true`) saved the other fields
and left the run at "Waiting for input". No message said why.

## Expected

When the step is saved but no change branch accepts it, the form says so and
keeps the values.
