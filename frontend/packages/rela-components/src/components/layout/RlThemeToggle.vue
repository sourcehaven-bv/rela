<script setup lang="ts">
/**
 * Picks the colour theme: follow the system, or force light or dark.
 *
 * Three choices rather than a switch, because `dark.css` defines three states
 * and the third is the one a user starts in. A binary control has to pick a
 * side on first paint for someone who has never expressed a preference, and
 * then gives them no way back to "whatever the OS says".
 *
 * Applying the choice is this component's job: the class names are the
 * library's contract with its own stylesheet, and an app writing them by hand
 * is reimplementing a detail that can drift. Remembering the choice is not.
 *
 * An app with its own dark palette has to answer `prefers-color-scheme` as
 * well, because `system` writes no class for its CSS to key off. An app that
 * instead resolves the OS in JavaScript and writes `.dark` itself ends up
 * writing the same two classes this does. That is safe, because both write
 * the same thing for the same choice, but it means the app is the one that
 * has to get `system` right: write neither class, or it pins the preference
 * it is meant to follow.
 *
 * Where a preference is stored is the app's decision, so bind `v-model` to
 * wherever it keeps one. Left unbound the control still works, and the choice
 * lasts as long as the page.
 */
import { computed, onMounted, ref, watch } from 'vue'
import RlSegmentedControl, { type SegmentOption } from '../data/RlSegmentedControl.vue'

export type ThemeChoice = 'system' | 'light' | 'dark'

const props = withDefaults(
  defineProps<{
    /**
     * The chosen theme. Left unbound the control keeps the choice itself,
     * for the whole page rather than beyond it.
     */
    modelValue?: ThemeChoice
    /** Accessible name for the group. Shown only when `iconOnly` is off. */
    label?: string
    /** Shows only the icons, for a collapsed rail or a tight footer. */
    iconOnly?: boolean
    size?: 'sm' | 'md'
    /**
     * Element the theme classes are written to. Defaults to the document
     * root, which is what `dark.css` selects on.
     */
    target?: HTMLElement | null
  }>(),
  { modelValue: undefined, label: 'Colour theme', iconOnly: false, size: 'sm', target: null },
)

const emit = defineEmits<{ 'update:modelValue': [value: ThemeChoice] }>()

/*
 * Uncontrolled unless bound. Tracking the choice internally keeps the control
 * honest when no `v-model` is given: otherwise the classes change but
 * `aria-checked` stays on the old option, so the theme visibly switches while
 * a screen reader still announces the previous one as selected.
 */
const inner = ref<ThemeChoice>(props.modelValue ?? 'system')
const current = computed<ThemeChoice>(() => props.modelValue ?? inner.value)

const options: SegmentOption[] = [
  { value: 'system', label: 'System', icon: 'monitor' },
  { value: 'light', label: 'Light', icon: 'sun' },
  { value: 'dark', label: 'Dark', icon: 'moon' },
]

/*
 * `system` is the absence of both classes rather than a third one: the
 * stylesheet reads `prefers-color-scheme` on its own once nothing has
 * overridden it. Both are cleared before one is set, which also makes this
 * safe against an element an app has already written to.
 */
function apply(choice: ThemeChoice) {
  const el = props.target ?? document.documentElement
  el.classList.remove('light', 'dark')
  if (choice !== 'system') el.classList.add(choice)
}

onMounted(() => apply(current.value))
watch(current, apply)

function select(value: string) {
  const choice = value as ThemeChoice
  inner.value = choice
  emit('update:modelValue', choice)
}
</script>

<template>
  <RlSegmentedControl
    class="rl-theme-toggle"
    :model-value="current"
    :options="options"
    :label="label"
    :size="size"
    :icon-only="iconOnly"
    @update:model-value="select"
  />
</template>

<style scoped>
/* Fills the band it sits in, such as the sidebar footer. */
.rl-theme-toggle { display: flex; }
.rl-theme-toggle :deep(.rl-segmented__option) { flex: 1; }
</style>
