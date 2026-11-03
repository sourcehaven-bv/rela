---
id: RR-DMR07Y
type: review-response
title: 'Recommendation on composer entry of options: defer it'
finding: 'The PoC need is agent-authored options (plan line 721). The textarea path adds toPropValue/fromPropValue/initialValues changes, a new RlChannelField mode and a hint, and diverges from the only existing free-text list convention in the SPA (comma separation, frontend/src/components/forms/TaskItemConversions.vue:379; data-entry uses RlMultiSelect for lists, src/widgets/registry.ts:28). Recommend: in this ticket the composer does not render list properties (carry them through edits unchanged, which the carryProp fix gives), and a follow-up picks the widget. Remove the composer half of AC 12.'
severity: minor
resolution: 'Composer entry of lists is removed: the composer does not render list properties, edits carry them through via the carryProp fix, and AC 12 no longer has a composer half.'
status: addressed
---
