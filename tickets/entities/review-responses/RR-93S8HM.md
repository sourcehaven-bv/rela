---
id: RR-93S8HM
type: review-response
title: No warning when decision_button can never show because every entry has a preset that overrides it
finding: 'buttonoptions.go:184-217 (effect) warns about a confirm with no effect only when the type has a required property. Approach 10 of the plan says the load warns whenever it can know the option reaches no button. The load can also know this case: every on.<type> entry in every state has a button: (so no type button exists), and every preset sets its own confirm (or style). The decision_button key then has no effect and no warning appears. Fix: in effect, compute whether any person-clickable entry lacks a button. When none does, warn per key for each key that all presets set themselves, with the same ''has no effect'' wording. Add two cases to TestTypeButton_Checks.'
severity: minor
resolution: effect now tracks whether any person-clickable entry lacks a button; when none does, it warns per key (confirm, style) that every preset sets itself, with 'has no effect' wording. The old required-property warning runs only when a type button exists. Existing 'every preset sets its own confirm' case updated to the new message; added a style case and a case that warns only for the key every preset sets. Load-check list in GUIDE-realtime updated.
status: addressed
---
