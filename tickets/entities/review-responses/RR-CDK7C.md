---
id: RR-CDK7C
type: review-response
title: Triple HTMLElement narrowing
finding: The instanceof HTMLElement check is duplicated in triggerActionFromClick, handleKeydown, and the executeAction caller chain. Could be DRY'd by widening triggerAction's parameter to Event | HTMLElement | null and narrowing once. Three lines, not blocking.
severity: nit
resolution: 'Consolidated HTMLElement narrowing into a single resolveTriggerEl(source: Event | HTMLElement | null | undefined) helper in useListActions.ts. triggerAction now accepts the wider Event | HTMLElement | null type and narrows once. EntityList.vue dropped its triggerActionFromClick wrapper and now does @click="(e) => triggerAction(id, config, e)" directly. Keyboard handler passes the keydown event in the same way.'
status: addressed
---
