<script setup lang="ts">
// The input box comes from the library's global `.rl-control`
// (rl/styles/control.css, imported by styles/rl.css): border, radius, focus
// ring, and the invalid and disabled visuals. It is declared globally rather
// than per component precisely so a host rendering a native element can reach
// it, which is what this widget is -- so this widget carries no styles of its
// own apart from the auto-grow rule.
//
// The invalid visual is driven by `aria-invalid` there, so the old error class
// is gone: the attribute now carries both the appearance and the announcement,
// where the class did the first and nothing for the second.
//
// The box grows with its text, as RlTextarea's `autoGrow` does, rather than
// reserving empty rows and showing a drag grip. RlTextarea itself cannot be
// used here: it renders its own field shell (see FieldShell.vue).
import { nextTick, onMounted, ref, watch } from 'vue'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)
const field = ref<HTMLTextAreaElement | null>(null)

// Reset before measuring, or the box could grow but never shrink back.
function resize() {
  const el = field.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
  resize()
}

// A value set from outside, such as a form reset or a loaded entity.
watch(stringValue, () => nextTick(resize))
onMounted(resize)
</script>

<template>
  <span v-if="mode === 'display'" class="display-value">{{ stringValue }}</span>
  <textarea
    v-else
    :id="id"
    ref="field"
    class="rl-control rl-control--multiline textarea-widget"
    :aria-describedby="describedBy"
    :aria-invalid="invalid || undefined"
    :value="stringValue"
    :placeholder="placeholder"
    :disabled="disabled"
    rows="3"
    @input="onInput"
  />
</template>

<style scoped>
.textarea-widget {
  resize: none;
  overflow: hidden;
}
</style>
