---
id: TKT-CHLAJ
type: ticket
title: Multi-step (wizard) forms with conditional steps
kind: enhancement
priority: medium
effort: m
status: planning
---

Add an optional wizard layout to the data-entry form config. A form can declare
ordered, titled steps, each with its own subset of fields/relations. Steps and
fields support `visible_when` / `required_when` conditions referencing earlier
field values (reusing the existing property-filter / when-then mechanism where
possible). Validation runs per step on "next", and the whole form on submit. The
current step is encoded in the URL (deep-link / refresh-safe), like the current
filter/sort URL-sync. Single-page forms keep working unchanged; wizard is opt-in
per form, and ideally the same field definitions render either way.

## Motivating case

A GDPR processing register (OpenVWR) walks through ~15 steps (Name → Controller
→ Processor → Recipient → Purpose & legal basis → Data subjects → Automated
decision-making → Systems → Security → Transfers outside EU → DPIA → Contact
person → Documents → Remarks → Publish). Several steps are conditional — e.g.
the "Processor" fields only become required when a "has processors" toggle is
on, and the DPIA step is a WP248 decision tree where each question only appears
if the previous one was answered "no".

This is a generic feature — onboarding flows, structured intake, surveys, any
long form — not just processing registers.

## Acceptance criteria

1. A data-entry form can be configured with ordered, titled steps.
2. A step or field can be shown/hidden and made required/optional based on the value of an earlier field in the same form.
3. Next/back navigation works; per-step validation blocks "next" on invalid input; full validation runs on submit.
4. The current step is encoded in the URL so refresh/deep-link returns to it.
5. Single-page forms keep working unchanged; wizard mode is opt-in.

## Reference (OpenVWR)

- `src/cms/app/Filament/Forms/Components/ProcessingRecordWizard.php` — thin wrapper over Filament's `Wizard` (`skippable`, `persistStepInQueryString`).
- Step definitions per register in the `...ResourceFormSchemas.php` files; conditional fields via toggles + `FormHelper::isFieldEnabled(...)`.
- Users switch steps-vs-one-page via a `register_layout` preference on the same schema.
