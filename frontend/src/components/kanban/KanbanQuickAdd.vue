<script setup lang="ts">
/**
 * The quick-add input at the foot of a kanban column or swimlane cell
 * (TKT-IP4273S). Enter hands the trimmed title to the board, which does the
 * create; this component only owns the text, the busy state and the error.
 *
 * The board passes `submit` as a function that resolves to whether the card
 * was created. On success the text clears and the input keeps focus, so a
 * facilitator can type the next card straight away. On failure the text
 * stays, so nothing typed is lost, and the error shows under the input.
 *
 * While a create is out the input is read-only rather than disabled: a
 * disabled input drops focus, and the next card would need a click.
 */
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RlTextField from 'rela-components/components/form/RlTextField.vue'

const props = defineProps<{
  /** The accessible name, naming the column and the lane. */
  label: string
  /** Creates the card. Resolves to an error message, or '' when it worked. */
  submit: (title: string) => Promise<string>
}>()

const { t } = useI18n()
const text = ref('')
const busy = ref(false)
const error = ref('')

async function onEnter(event: KeyboardEvent) {
  // An IME composition commits with Enter; that Enter is not a submit.
  if (event.isComposing || busy.value) return
  const title = text.value.trim()
  if (!title) return
  busy.value = true
  try {
    error.value = await props.submit(title)
    if (!error.value) text.value = ''
  } finally {
    busy.value = false
  }
}

function onEscape() {
  text.value = ''
  error.value = ''
}
</script>

<template>
  <RlTextField
    v-model="text"
    class="kanban-quick-add"
    size="sm"
    :label="label"
    label-hidden
    :placeholder="t('kanban.quickAddPlaceholder')"
    :readonly="busy"
    :error="error || undefined"
    autocomplete="off"
    data-testid="kanban-quick-add"
    @keydown.enter.prevent="onEnter"
    @keydown.esc.stop="onEscape"
  />
</template>

<style scoped>
.kanban-quick-add {
  margin-top: var(--rl-space-2);
}
</style>
