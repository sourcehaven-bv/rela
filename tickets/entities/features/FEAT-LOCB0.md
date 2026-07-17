---
id: FEAT-LOCB0
type: feature
title: Multi-step (wizard) forms with conditional steps
summary: 'Optional wizard layout for data-entry forms: ordered titled steps, next/back navigation, per-step validation, conditional visible_when/required_when on steps and fields, and step encoded in the URL. Wizard is opt-in per form; single-page stays the default and the same field definitions render either way.'
description: |-
    rela's data-entry forms are single-page today: a flat list of fields + relations. Data-rich guided entry (GDPR processing registers, onboarding flows, structured intake, surveys) needs a multi-step wizard where the form is split into ordered steps, the user moves next/back, and some steps/fields only appear based on earlier answers.

    Scope of the feature:
    - A form config can declare ordered, titled steps, each with its own subset of fields/relations.
    - Steps and individual fields support visible_when / required_when conditions referencing earlier field values, reusing the existing property-filter / when-then mechanism where possible.
    - Validation runs per step on 'next'; the whole form validates on submit.
    - The current step is encoded in the URL (deep-link / refresh-safe), like the existing filter/sort URL-sync.
    - Single-page form remains the default; wizard is opt-in per form. The same field definitions should be able to render either as one page or as steps (the user/schema picks the layout).

    Reference implementation (OpenVWR / Filament Wizard): thin wrapper over Filament's Wizard with persistStepInQueryString; per-register step definitions; conditional fields via toggles + FormHelper::isFieldEnabled; a register_layout preference switches steps-vs-one-page on the same schema.
priority: medium
status: proposed
---
