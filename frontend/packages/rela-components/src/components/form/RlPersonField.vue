<script setup lang="ts">
/**
 * Picks a person, by typing their name or by arrowing the list.
 *
 * Not a use of `RlOptionSelect` with a person in the slot, though that is the
 * obvious move and was the first attempt. That control keeps real focus on
 * its trigger, which is right for a list of statuses and fatal here: a list
 * of people is long enough to need filtering, and a search box inside a panel
 * that refuses focus can never be typed into. So the filter lives in the
 * trigger, as a real input, and the trigger is the combobox.
 *
 * Typing filters on the name and on the secondary line both, because two
 * people called Jan are told apart by the line under the name, and that line
 * is what the user will reach for.
 *
 * Clearing is a listed choice rather than a button in the corner. "Unassign"
 * is a decision with the same weight as picking someone, and a small x beside
 * a field is a reliable way to hide it from anyone not using a mouse.
 */
import { computed, nextTick, ref, useId, watch } from 'vue'
import { useAnchoredPanel } from '../../composables/useAnchoredPanel'
import { useOverlayStack } from '../../composables/useOverlayStack'
import RlFieldShell from './RlFieldShell.vue'
import RlPerson from '../data/RlPerson.vue'
import RlIcon from '../common/RlIcon.vue'
import type { Person } from '../data/types'

const props = withDefaults(
  defineProps<{
    /** The chosen person's id, or empty for unassigned. */
    modelValue?: string
    label: string
    people: Person[]
    hint?: string
    error?: string
    required?: boolean
    disabled?: boolean
    labelHidden?: boolean
    placeholder?: string
    /**
     * Whether the list can be emptied, shown as a first "Unassigned" row. Off
     * for a field that must name someone.
     */
    clearable?: boolean
    /** Wording for the empty state and for the clearing row. */
    emptyLabel?: string
    /**
     * Whether the field filters the list itself. Off where the caller is
     * searching a directory and `people` is already the answer, in which case
     * listen to `update:query`.
     */
    filter?: boolean
    /** Shown in the panel when nothing matches. */
    noMatchLabel?: string
    /**
     * `inline` renders the value bare and only draws the chrome on hover and
     * focus, matching `RlOptionSelect` and `RlMultiSelect`. For a person
     * sitting in a record rather than in a form. It drops the field shell
     * with it; `label` stays, for assistive tech.
     */
    variant?: 'control' | 'inline'
  }>(),
  {
    modelValue: '',
    required: false,
    disabled: false,
    labelHidden: false,
    placeholder: 'Unassigned',
    clearable: true,
    emptyLabel: 'Unassigned',
    filter: true,
    noMatchLabel: 'No matching people',
    variant: 'control',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** The typed filter changed, for a caller searching a directory. */
  'update:query': [value: string]
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const input = ref<HTMLInputElement | null>(null)
const query = ref('')
const activeIndex = ref(0)
const uid = useId()

/*
 * Focus belongs in the input here, unlike the other two pickers: the input is
 * the thing being typed into. The panel still must not steal a press, or the
 * input would blur and tear the panel down before the click landed.
 */
const { floatingStyles, containsTarget, panelHandlers } = useAnchoredPanel(root, panel, {
  placement: 'bottom-start',
  matchWidth: props.variant === 'control',
  keepFocusOnTrigger: true,
})

const { push, pop } = useOverlayStack({ scrollLock: false })

const selected = computed(() => props.people.find((person) => person.id === props.modelValue))

const matches = computed(() => {
  if (!props.filter) return props.people
  const needle = query.value.trim().toLowerCase()
  if (!needle) return props.people
  return props.people.filter((person) =>
    `${person.name} ${person.secondary ?? ''}`.toLowerCase().includes(needle),
  )
})

/*
 * The clearing row is part of the list rather than a thing beside it, so one
 * set of arrow keys reaches every choice. Index 0 when it is there, which is
 * why everything below counts rows and not people.
 */
const rows = computed<(Person | null)[]>(() =>
  props.clearable && !query.value.trim() ? [null, ...matches.value] : [...matches.value],
)

const optionId = (index: number) => `${uid}-option-${index}`

const isDisabled = (index: number) => rows.value[index]?.disabled === true

function nextEnabled(from: number, delta: number) {
  const count = rows.value.length
  if (!count) return -1
  for (let step = 0; step < count; step += 1) {
    const index = (from + delta * step + count * count) % count
    if (!isDisabled(index)) return index
  }
  return -1
}

async function show() {
  if (props.disabled || open.value) return
  query.value = ''
  const current = rows.value.findIndex((row) => row?.id === props.modelValue)
  activeIndex.value = current === -1 ? 0 : current
  open.value = true
  push()
  await nextTick()
  scrollActiveIntoView()
}

function hide() {
  if (!open.value) return
  open.value = false
  query.value = ''
  pop()
}

function choose(index: number) {
  if (index < 0 || index >= rows.value.length || isDisabled(index)) return
  const row = rows.value[index]
  hide()
  emit('update:modelValue', row?.id ?? '')
}

function scrollActiveIntoView() {
  panel.value
    ?.querySelector(`#${CSS.escape(optionId(activeIndex.value))}`)
    ?.scrollIntoView({ block: 'nearest' })
}

watch(activeIndex, () => {
  if (open.value) scrollActiveIntoView()
})

/* A new filter means a new list, so the cursor goes back to the top of it. */
watch(query, (value) => {
  activeIndex.value = Math.max(nextEnabled(0, 1), 0)
  emit('update:query', value)
})

function setActive(index: number) {
  if (index !== -1) activeIndex.value = index
}

function move(delta: number) {
  if (!rows.value.length) return
  setActive(nextEnabled(activeIndex.value + delta, delta))
}

function onKeydown(event: KeyboardEvent) {
  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      if (open.value) move(1)
      else void show()
      break
    case 'ArrowUp':
      event.preventDefault()
      if (open.value) move(-1)
      else void show()
      break
    case 'Home':
      if (!open.value) return
      event.preventDefault()
      setActive(nextEnabled(0, 1))
      break
    case 'End':
      if (!open.value) return
      event.preventDefault()
      setActive(nextEnabled(rows.value.length - 1, -1))
      break
    case 'Enter':
      /*
       * Prevented whether or not the list is open, so Enter in the filter
       * never submits the form the field is sitting in.
       */
      event.preventDefault()
      if (open.value) choose(activeIndex.value)
      else void show()
      break
    case 'Escape':
      if (!open.value) return
      // Stopped so closing the list does not also close a surrounding layer.
      event.stopPropagation()
      hide()
      break
    case 'Tab':
      if (open.value) hide()
      break
  }
}

function onFocusout(event: FocusEvent) {
  // The panel is teleported out of `root`, so the composable answers for both.
  if (containsTarget(root.value, event.relatedTarget as Node | null)) return
  hide()
}

defineExpose({ open: show, close: hide, focus: () => input.value?.focus() })
</script>

<template>
  <!--
    One shell either way, rather than a second copy of the trigger under a
    `v-if`. Inline hides the label and drops the hint and the error, which
    leaves the shell rendering nothing but a visually-hidden `label` pointing
    at the input: a form's furniture gone, the control's name kept.
  -->
  <RlFieldShell
    v-slot="control"
    :label="label"
    :label-hidden="labelHidden || variant === 'inline'"
    :hint="variant === 'inline' ? undefined : hint"
    :error="variant === 'inline' ? undefined : error"
    :required="required"
  >
    <div
      ref="root"
      class="rl-person-field"
      :class="`rl-person-field--${variant}`"
      @keydown="onKeydown"
      @focusout="onFocusout"
    >
      <!--
        The value is drawn under the input rather than inside it: an input
        cannot hold an avatar, and swapping the face out for a bare string
        while typing would lose the thing the user was aiming at. The input
        lies over the top and carries the filter alone.
      -->
      <div class="rl-person-field__box" :class="{ 'rl-person-field__box--open': open }">
        <!--
          One line only: the box is one row tall, and the secondary line earns
          its place in the list, where it tells two people with the same name
          apart, not here where one has already been chosen.
        -->
        <RlPerson
          v-show="!query"
          :person="selected"
          size="sm"
          secondary-hidden
          :empty-label="placeholder"
          class="rl-person-field__value"
          aria-hidden="true"
        />

        <input
          :id="control.id"
          ref="input"
          v-model="query"
          type="text"
          class="rl-person-field__input"
          role="combobox"
          :aria-expanded="open"
          aria-haspopup="listbox"
          aria-autocomplete="list"
          :aria-controls="open ? `${uid}-list` : undefined"
          :aria-activedescendant="open ? optionId(activeIndex) : undefined"
          :aria-describedby="control.describedBy"
          :aria-invalid="control.invalid || undefined"
          :required="control.required"
          :disabled="disabled"
          @mousedown="show()"
        />

        <RlIcon name="chevron-down" :size="14" class="rl-person-field__arrow" aria-hidden="true" />
      </div>

      <!-- Teleported so a scrolling table or a panel cannot clip the list. -->
      <Teleport to="body">
        <div
          v-if="open"
          :id="`${uid}-list`"
          ref="panel"
          class="rl-panel rl-panel--options rl-person-field__list"
          :style="floatingStyles"
          role="listbox"
          :aria-label="label"
          v-bind="panelHandlers"
        >
          <div
            v-for="(row, index) in rows"
            :id="optionId(index)"
            :key="row?.id ?? '_none'"
            class="rl-option-select__option"
            :class="{
              'rl-option-select__option--active': index === activeIndex && !isDisabled(index),
              'rl-option-select__option--selected': (row?.id ?? '') === modelValue,
              'rl-option-select__option--disabled': isDisabled(index),
            }"
            role="option"
            :aria-selected="(row?.id ?? '') === modelValue"
            :aria-disabled="isDisabled(index) || undefined"
            @click="choose(index)"
            @mousemove="isDisabled(index) || (activeIndex = index)"
          >
            <RlPerson :person="row ?? undefined" size="sm" :empty-label="emptyLabel" />
          </div>

          <p v-if="!rows.length" class="rl-person-field__empty">{{ noMatchLabel }}</p>
        </div>
      </Teleport>
    </div>
  </RlFieldShell>
</template>

<style scoped>
.rl-person-field__box {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-1) var(--rl-space-2);
  border: 1px solid var(--rl-color-border-strong);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg);
  cursor: text;
}

/*
 * Inline holds the chrome back until it is wanted, matching the other two
 * pickers. Transparent rather than absent, so nothing shifts when it appears.
 */
.rl-person-field--inline .rl-person-field__box {
  border-color: transparent;
  background: transparent;
}

/*
 * Inline hugs its value rather than filling the row, matching
 * `RlOptionSelect`. Stretched, the chrome would light up under a pointer
 * nowhere near the name, which reads as a stray highlight rather than as an
 * invitation to edit.
 */
.rl-person-field--inline {
  display: inline-flex;
  max-width: 100%;
}


.rl-person-field--inline .rl-person-field__box:hover,
.rl-person-field--inline .rl-person-field__box--open {
  border-color: var(--rl-color-border);
  background: var(--rl-color-bg-hover);
}

.rl-person-field--inline .rl-person-field__arrow {
  opacity: 0;
  transition: opacity var(--rl-duration-fast) var(--rl-ease);
}

.rl-person-field--inline .rl-person-field__box:hover .rl-person-field__arrow,
.rl-person-field--inline .rl-person-field__box--open .rl-person-field__arrow,
.rl-person-field--inline .rl-person-field__box:focus-within .rl-person-field__arrow {
  opacity: 1;
}

@media (prefers-reduced-motion: reduce) {
  .rl-person-field--inline .rl-person-field__arrow { transition: none; }
}

.rl-person-field__box:focus-within {
  border-color: var(--rl-color-focus);
  box-shadow:
    0 0 0 var(--rl-focus-ring-gap) var(--rl-color-bg),
    0 0 0 calc(var(--rl-focus-ring-gap) + var(--rl-focus-ring-width)) var(--rl-color-focus);
}

/*
 * The value stays in the flow and the input lies over it, rather than the
 * other way round. It is the value that has to size the box: an input has an
 * intrinsic width of about twenty characters and no knowledge of the name it
 * is covering, so letting it size the box makes an inline field a fixed-width
 * slot in the middle of a record.
 */
.rl-person-field__value {
  min-width: 0;
  pointer-events: none;
}

/*
 * Over the value, filling the box. Transparent while it is empty, so the
 * face underneath shows through; typing hides the value and leaves the text.
 */
.rl-person-field__input {
  position: absolute;
  inset: 0;
  width: 100%;
  padding: 0 var(--rl-space-8) 0 var(--rl-space-2);
}

/* Bare: the box around it is the control, so a second border would draw a
   field inside a field. */
.rl-person-field__input {
  border: none;
  background: none;
  font: inherit;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text);
}

.rl-person-field__input:focus { outline: none; }

/*
 * Above the input, which covers the whole box. It is not a control, so it
 * passes presses through to the input underneath rather than swallowing a
 * click aimed at the right-hand end of the field.
 */
.rl-person-field__arrow {
  position: relative;
  z-index: 1;
  flex: none;
  margin-left: auto;
  color: var(--rl-color-text-muted);
  pointer-events: none;
}

.rl-person-field__empty {
  margin: 0;
  padding: var(--rl-space-2);
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
}
</style>
