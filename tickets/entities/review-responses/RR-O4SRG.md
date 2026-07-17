---
id: RR-O4SRG
type: review-response
title: Wizard hidden-branch pruning bypassed on create path (revealed-then-hidden field persists)
finding: handleSubmit computed a wizard-pruned properties set, but the create branch overwrote it with visibleWritablePropertiesForCommit() which had no knowledge of wizard.activeProperties and unconditionally preserved userTouched keys. So reveal -> fill -> hide -> submit persisted the hidden-branch value on create (the primary wizard flow). Server is not a backstop (visible_when is client-only). The e2e masked it by never entering the conditional field.
severity: critical
resolution: Extracted pruneWizardHidden() applied by BOTH submit paths — on create it now composes with the affordance prune (pruneWizardHidden(visibleWritablePropertiesForCommit())), and a visible_when-hidden key wins over the userTouched-preserve rule. Added a reveal->fill->hide->submit e2e test on the create path that asserts the value is dropped (was the missing coverage). Refined so only wizard-managed-but-inactive keys are dropped (see RR for finding 3).
status: addressed
---
