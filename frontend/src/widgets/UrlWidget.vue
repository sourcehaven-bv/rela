<script setup lang="ts">
// Renders a string property that holds a URL (TKT-0101IO4). Display mode is a
// link when the value's scheme is http, https or mailto (see linkHref), and
// plain text otherwise, so a stored `javascript:` or `data:` value never
// becomes a clickable target.
//
// The same widget serves the detail page, view sections and the dense
// surfaces. On a dense surface the link may sit inside a control that opens
// on click: a list row, a kanban card, a calendar chip, a linked table cell,
// a nested row's <summary>. `@click.stop` keeps a click on the link from also
// opening that control; the kanban card skips a click on a nested link.
//
// Edit mode is a text input with the URL keyboard. Not `type="url"`: a form
// submit would then refuse an entity whose stored value is not a full URL
// (such as `example.com`), even when the user did not touch the field.
import { computed } from 'vue'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'
import { linkHref, opensInNewTab } from '@/utils/linkHref'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

// A view-section card falls back to the server's string list when the raw
// value is absent. The property is single-valued (the schema refuses a link
// format on a list), so that list holds at most one element.
const text = useStringValue(() =>
  Array.isArray(props.modelValue) ? props.modelValue[0] : props.modelValue
)

const href = computed(() => (props.mode === 'display' ? linkHref(text.value) : undefined))
const newTab = computed(() => href.value !== undefined && opensInNewTab(href.value))

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}
</script>

<template>
  <a
    v-if="href"
    class="display-value url-link"
    :href="href"
    :target="newTab ? '_blank' : undefined"
    rel="noopener noreferrer"
    @click.stop
    >{{ text }}</a
  >
  <span v-else-if="mode === 'display'" class="display-value">{{ text }}</span>
  <input
    v-else
    :id="id"
    type="text"
    inputmode="url"
    class="rl-control"
    :aria-describedby="describedBy"
    :aria-invalid="invalid || undefined"
    :value="text"
    :placeholder="placeholder"
    :disabled="disabled"
    @input="onInput"
  />
</template>

<style scoped>
.url-link {
  overflow-wrap: anywhere;
}
</style>
