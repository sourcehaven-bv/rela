---
id: RR-IS01DT
type: review-response
title: The async template fetch overwrote a body the user had started writing
finding: loadTemplates() fetches templates asynchronously and auto-applies the first one, and applyTemplate() assigned content.value unconditionally. The form is interactive before the fetch resolves, so anything the user had begun writing was replaced by the template body. Pre-existing on develop — the same e2e test fails there with the old EasyMDE editor — but it surfaced through the entity-reference picker and would have been read as a Milkdown regression.
severity: significant
resolution: applyTemplate takes a preserveUserInput flag, set on the automatic application after the fetch and not on an explicit choice from the template dropdown, where replacing the content is the intent.
status: addressed
---

## Finding

Not introduced by this change: `git stash` + `git checkout develop` reproduces
the same e2e failure with the EasyMDE editor in place. Recorded here because the
work surfaced it and it is a silent data-loss bug.

`loadTemplates()` awaits `getTemplates(...)` then calls `applyTemplate(...)`,
which did `content.value = template.content` unconditionally. The form is usable
before the fetch lands.

## Why it is fixed here rather than deferred

The nearby fixes are all about the editor never losing what a user wrote.
Leaving a known path that discards the body — reachable through the very picker
this ticket touches — would have meant the e2e suite stayed red and the next
person would have had to re-diagnose it as a Milkdown problem.

## Resolution

`applyTemplate(template, preserveUserInput = false)`. The automatic application
after the fetch passes true; `selectTemplate` (an explicit dropdown choice)
keeps the existing replace behaviour, because that is what picking a template
means.

Scoped to `content` deliberately. Properties are prefilled before the form is
usable, so they do not have the same exposure, and widening the guard would have
changed template behaviour beyond what this ticket verified.
