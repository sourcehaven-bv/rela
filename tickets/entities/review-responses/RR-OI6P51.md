---
id: RR-OI6P51
type: review-response
title: Staged files bypass wizard-hidden pruning, persisting a revealed-then-hidden branch
finding: |-
    `uploadStagedFiles` (DynamicForm.vue:499) iterates `stagedFiles.value` raw. Properties go through `pruneWizardHidden(visibleWritablePropertiesForCommit())` at line 1081; staged files go through nothing.

    Concrete failure: a wizard whose step 2 has `visible_when: form.needs_evidence == true` and contains a `file` property. The user ticks the box, reveals step 2, stages evidence.pdf, goes back, unticks. `pruneWizardHidden` correctly drops the PROPERTY from the create payload — and then uploadStagedFiles uploads the file anyway, and the server stamps that same property via stampPropertyNames. The revealed-then-hidden branch persists, which is exactly what TKT-CHLAJ exists to prevent, and the existing e2e 'a revealed-then-hidden field is NOT persisted on create' does not cover file properties.

    Fix: filter the upload loop against the same active/managed sets (wizard.activeProperties / wizard.managedProperties, as used at DynamicForm.vue:900):

      if (managed.has(property) && !active.has(property)) continue

    Also worth staging-side pruning so the bytes are never uploaded at all.
severity: significant
resolution: 'uploadStagedFiles now applies the same wizard pruning the property payload gets: it reads wizard.activeProperties / wizard.managedProperties and skips any property that is managed-but-inactive, so a file staged on a step the user then hides is never uploaded and the server never stamps the property. Pinned by ''does not upload a file staged under a then-hidden wizard branch'', which drives the real wizard step navigation (reveal → Next step → stage → Prev step → hide → submit) and asserts uploadAttachment is never called.'
status: addressed
---

## Finding

The two pruning systems the plan reconciled for *properties* do not cover
*staged files*.

- Properties: `payload.properties = pruneWizardHidden(visibleWritablePropertiesForCommit())`
(`DynamicForm.vue:1081`), with the active/managed logic at line 900.
- Staged files: `for (const [property, files] of Object.entries(stagedFiles.value))`
(`DynamicForm.vue:499`) — unfiltered.

## Failure scenario

A wizard step gated on `visible_when: form.needs_evidence == true`, containing a
`file` property:

1. User ticks `needs_evidence` → step 2 reveals
2. Stages `evidence.pdf`
3. Goes back, unticks the box → step 2 hides

`pruneWizardHidden` drops `evidence` from the create payload — correct. Then
`uploadStagedFiles` uploads the file regardless, and the server stamps
`evidence` itself in `stampPropertyNames`. The hidden branch persists.

This is precisely what TKT-CHLAJ prevents for ordinary properties. The existing
e2e case "a revealed-then-hidden field is NOT persisted on create" passes
because it uses no file property.

## Fix

```js
const active = wizard.activeProperties.value
const managed = wizard.managedProperties.value
for (const [property, files] of Object.entries(stagedFiles.value)) {
  if (managed.has(property) && !active.has(property)) continue
  ...
}
```

Pruning at stage time as well would avoid uploading the bytes at all, but the
commit-time filter is the correctness boundary.
