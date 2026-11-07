<script setup lang="ts">
// FieldHint is the info button next to a field label and the panel it opens:
// a field's `hint`, for guidance too long to show below the field at all
// times. It renders the button and the panel as siblings, so place it inside
// a wrapping flex row after the label (`.field-label-row`); the panel takes a
// full row of its own.
//
// A disclosure rather than a hover tooltip: a tooltip cannot be reached by
// touch or keyboard, and a long text with a link needs to stay open.
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { Info } from 'lucide-vue-next'
import MarkdownHelp from './MarkdownHelp.vue'

const props = defineProps<{ hint: string; label?: string }>()

const { t } = useI18n()
const open = ref(false)
const panelId = `${useId()}-hint`
const buttonLabel = computed(() =>
  props.label ? t('form.hint.toggleFor', { label: props.label }) : t('form.hint.toggle')
)
</script>

<template>
  <button
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
  <MarkdownHelp v-show="open" :id="panelId" class="field-hint" :text="hint" />
</template>

<style scoped>
.field-hint-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
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
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 2px;
}

.field-hint {
  flex-basis: 100%;
  font-size: 13px;
  font-weight: 400;
  color: var(--rl-color-text);
  background: var(--rl-color-bg-sunken);
  border-left: 3px solid var(--rl-color-border);
  padding: 8px 12px;
}
</style>
