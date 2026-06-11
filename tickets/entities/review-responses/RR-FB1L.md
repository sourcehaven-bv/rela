---
id: RR-FB1L
type: review-response
title: 'S3: Per-field error chrome — without FieldShell, errors surface only as toast'
finding: |
  SectionEditForm not wrapping widgets in FieldShell means per-field error pills (currently shown by FieldShell L37 `<p v-if="error" class="field-error">`) are absent. User edits a cell, gets a 422, sees a vague toast, has to guess which cell.
severity: significant
status: addressed
resolution: |
  Adopt L3 from review: reuse FieldShell with `:label="undefined"` (the `<dt>` is the label) and `:error="autoSave.fieldErrors[prop]"`. SectionEditForm's per-cell render becomes:

  ```vue
  <dt>{{ field.label }}</dt>
  <dd>
    <FieldShell :field-id="..." :error="autoSave.fieldErrors[field.property]" :label="undefined">
      <component :is="widget" mode="edit" v-model="formData[field.property]" ... />
    </FieldShell>
  </dd>
  ```

  Per-field validation errors land inline at the cell. Reuses existing chrome. Smaller diff than rolling new error styling. PLAN test plan adds: "422 PATCH response on field X shows error pill below cell X, not elsewhere."
---
