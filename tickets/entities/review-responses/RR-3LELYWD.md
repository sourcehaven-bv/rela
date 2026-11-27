---
id: RR-3LELYWD
type: review-response
title: Typeahead shows the raw id as committed label
finding: EntityTargetSelect sets committedLabel to modelValue. Once the value is an id the typeahead placeholder would show the id instead of the title.
severity: significant
resolution: 'Plan: derive the committed label from the option whose value equals modelValue; fall back to the raw value for a deep-linked value with no candidate.'
status: addressed
---
