---
id: RR-JBQAC
type: review-response
title: Wizard validation scope skipped the affordance filter used by rendering (dead-end form)
finding: submitScopeFields and handleNext used wizard.visibleFieldsOf(step) without affordanceVisible, while rendering used visibleStepFields = visibleFieldsOf + affordanceVisible. A policy-hidden required (or required_when) field would be in validation scope but not rendered, so final submit/Next could fail on a field the user can't see or fill.
severity: significant
resolution: submitScopeFields and handleNext now use visibleStepFields(step) (which applies affordanceVisible), aligning the wizard validation scope with what renders — matching single-page validate() which iterates the affordance-filtered fields.value.
status: addressed
---
