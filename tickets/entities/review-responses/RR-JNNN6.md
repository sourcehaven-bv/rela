---
id: RR-JNNN6
type: review-response
title: Wizard with all-conditional steps could show a Submit over an empty form
finding: If every step's visible_when is currently false, currentStepDef is undefined, the panel is v-if-guarded away, but isLastStep (0 >= -1) is true so a Submit button rendered over an empty form.
severity: minor
resolution: The wizard footer is now gated on wizard.currentStepDef.value, and a 'No steps to display.' message renders when there are no visible steps. No misleading Submit.
status: addressed
---
