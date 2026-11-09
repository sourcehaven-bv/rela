<script setup lang="ts">
// FieldLabelRow is the row that holds a field's label (the default slot) and,
// when the field has a `hint`, the info button that opens it. The hint panel
// renders below the row, so the row never wraps: a long label wraps its own
// text beside a checkbox instead of dropping below it.
//
// A disclosure rather than a hover tooltip: a tooltip cannot be reached by
// touch or keyboard, and a long text with a link needs to stay open.
//
// The root is `display: contents`, so the row and the panel take part in the
// parent's layout (the column gap of `.form-field`) as if written inline.
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { Info } from 'lucide-vue-next'
import MarkdownHelp from './MarkdownHelp.vue'

const props = defineProps<{
  hint?: string
  // The field's label, for the button's accessible name.
  label?: string
  // Extra class on the row itself, such as `checkbox-wrapper`.
  rowClass?: string
}>()

const { t } = useI18n()
const open = ref(false)
const panelId = `${useId()}-hint`
const buttonLabel = computed(() =>
  props.label ? t('form.hint.toggleFor', { label: props.label }) : t('form.hint.toggle')
)
</script>

<template>
  <div class="field-label-block">
    <div class="field-label-row" :class="rowClass">
      <slot />
      <button
        v-if="hint"
        type="button"
        class="field-hint-toggle"
        :class="{ open }"
        :aria-expanded="open"
        :aria-controls="panelId"
        :aria-label="buttonLabel"
        :title="buttonLabel"
        data-testid="field-hint-toggle"
        @click="open = !open"
      >
        <Info :size="16" aria-hidden="true" />
      </button>
    </div>
    <MarkdownHelp v-if="hint" v-show="open" :id="panelId" class="field-hint" :text="hint" />
  </div>
</template>

<style scoped>
.field-label-block {
  display: contents;
}

.field-label-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.field-label-row > :slotted(label) {
  min-width: 0;
}

/* 24px target (WCAG 2.2 2.5.8) around a 16px icon. */
.field-hint-toggle {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  min-height: 24px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--rl-color-text-muted);
  line-height: 1;
  cursor: pointer;
}

.field-hint-toggle:hover,
.field-hint-toggle.open {
  color: var(--rl-color-accent);
}

.field-hint-toggle:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
}

.field-hint {
  font-size: 13px;
  font-weight: 400;
  color: var(--rl-color-text);
  background: var(--rl-color-bg-sunken);
  border-left: 3px solid var(--rl-color-border);
  padding: 8px 12px;
}
</style>
