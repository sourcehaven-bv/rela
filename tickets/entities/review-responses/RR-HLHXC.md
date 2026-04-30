---
id: RR-HLHXC
type: review-response
title: Single-global-instance alternative dismissed without enough rigor
finding: The plan rejected a single app-level ConfirmModal in App.vue. For this SPA-only codebase with no slotted content needs, a single global modal is genuinely simpler and removes per-callsite template plumbing forever. Either adopt it, or expand the rejection rationale beyond 'too much machinery.' Acceptable to keep the per-callsite pattern but document why properly.
severity: minor
resolution: 'Adopted: single-global-instance pattern. ConfirmModal mounted once in App.vue and driven by a singleton useConfirm composable. Existing per-callsite usage in EntityList/EntityDetail will be migrated for consistency (added as a follow-up task in this ticket scope).'
status: addressed
---
