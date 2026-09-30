<script setup lang="ts">
/**
 * Single-choice picker whose options are drawn rather than described.
 *
 * A native `select` can only hold text, so a status loses its colour and a
 * priority loses its badge the moment the list opens. This renders each
 * option through a slot instead, which is the whole reason it exists: the
 * list shows the same thing the closed value shows.
 *
 * Everything the native control gave for free has to be rebuilt, so it
 * follows the ARIA listbox pattern: arrows move the active option, Home and
 * End jump to the ends, Enter and Space choose, Escape closes without
 * choosing, and `aria-activedescendant` tells a screen reader which option is
 * current without moving real focus off the trigger.
 *
 * An option a rule forbids is shown rather than hidden, through
 * `isOptionDisabled`. A list that silently drops a choice leaves the user
 * looking for something that is not there; a greyed one says the choice
 * exists and is not theirs right now.
 */
import { nextTick, ref, useId, watch } from 'vue'
import { useAnchoredPanel } from '../../composables/useAnchoredPanel'
import { useOverlayStack } from '../../composables/useOverlayStack'
import RlIcon from '../common/RlIcon.vue'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    /** Values in list order. What each one looks like comes from the slot. */
    options: readonly string[]
    /** Names the control for assistive tech. */
    label: string
    placeholder?: string
    disabled?: boolean
    /**
     * Whether a single option cannot be chosen. A disabled option still
     * shows, greyed and marked `aria-disabled`, but the arrows skip it and
     * Enter and click do nothing on it.
     */
    isOptionDisabled?: (value: string) => boolean
    /** Matches the panel's width to the trigger's. */
    matchWidth?: boolean
    /**
     * `inline` renders the trigger bare, as the value alone, and only draws
     * the button chrome on hover and focus. For a value sitting in a detail
     * row or a table cell, where a permanent box on every field would read as
     * a form rather than as a record.
     *
     * The control is the same either way. Nothing is swapped in on click,
     * which is the point: the thing you hover is the thing that opens.
     */
    variant?: 'control' | 'inline'
  }>(),
  {
    modelValue: '',
    placeholder: 'Select',
    disabled: false,
    isOptionDisabled: () => false,
    matchWidth: false,
    variant: 'control',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** Closed without choosing. */
  cancel: []
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const uid = useId()

/** The option the keyboard is on, which is not yet the chosen one. */
const activeIndex = ref(0)

const { floatingStyles, containsTarget, panelHandlers } = useAnchoredPanel(root, panel, {
  placement: 'bottom-start',
  matchWidth: props.matchWidth,
  // Focus stays on the trigger and the options are plain divs, so the panel
  // must not let a press move focus off it.
  keepFocusOnTrigger: true,
})

const { push, pop } = useOverlayStack({ scrollLock: false })

const optionId = (index: number) => `${uid}-option-${index}`

const isDisabled = (index: number) => {
  const value = props.options[index]
  return value === undefined || props.isOptionDisabled(value)
}

/** The first option that can be chosen, searching outwards from `from`. */
function nextEnabled(from: number, delta: number) {
  const count = props.options.length
  for (let step = 0; step < count; step += 1) {
    const index = (from + delta * step + count * count) % count
    if (!isDisabled(index)) return index
  }
  return -1
}

async function show() {
  if (props.disabled || open.value) return
  // Opens on the current value, so the list starts where the user left it.
  const current = props.options.indexOf(props.modelValue)
  // A forbidden current value can still be the selected one, so the search
  // for somewhere to stand starts there and moves on if it cannot stay.
  const from = current === -1 ? 0 : current
  activeIndex.value = isDisabled(from) ? Math.max(nextEnabled(from, 1), 0) : from
  open.value = true
  push()
  await nextTick()
  scrollActiveIntoView()
}

function hide() {
  if (!open.value) return
  open.value = false
  pop()
}

function choose(index: number) {
  const value = props.options[index]
  if (value === undefined || isDisabled(index)) return
  hide()
  // Emitted after closing, so a listener that moves focus is not fighting the
  // panel being torn down.
  emit('update:modelValue', value)
}

function scrollActiveIntoView() {
  panel.value
    ?.querySelector(`#${CSS.escape(optionId(activeIndex.value))}`)
    ?.scrollIntoView({ block: 'nearest' })
}

watch(activeIndex, () => {
  if (open.value) scrollActiveIntoView()
})

/* Every option disabled leaves nowhere to stand, so the active option stays
   where it is rather than jumping to a choice that cannot be made. */
function setActive(index: number) {
  if (index !== -1) activeIndex.value = index
}

function move(delta: number) {
  if (!props.options.length) return
  setActive(nextEnabled(activeIndex.value + delta, delta))
}

function onKeydown(event: KeyboardEvent) {
  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      if (open.value) move(1)
      else show()
      break
    case 'ArrowUp':
      event.preventDefault()
      if (open.value) move(-1)
      else show()
      break
    case 'Home':
      if (!open.value) return
      event.preventDefault()
      setActive(nextEnabled(0, 1))
      break
    case 'End':
      if (!open.value) return
      event.preventDefault()
      setActive(nextEnabled(props.options.length - 1, -1))
      break
    case 'Enter':
    case ' ':
      event.preventDefault()
      if (open.value) choose(activeIndex.value)
      else show()
      break
    case 'Escape':
      if (!open.value) return
      // Stopped so closing the list does not also close a surrounding layer.
      event.stopPropagation()
      hide()
      emit('cancel')
      break
    case 'Tab':
      // Tab means "move on", so the list closes rather than trapping focus.
      if (open.value) {
        hide()
        emit('cancel')
      }
      break
  }
}

function onFocusout(event: FocusEvent) {
  // The panel is teleported out of `root`, so the composable answers for both.
  if (containsTarget(root.value, event.relatedTarget as Node | null)) return
  if (!open.value) return
  hide()
  emit('cancel')
}

defineExpose({ open: show, close: hide })
</script>

<template>
  <div ref="root" class="rl-option-select" @keydown="onKeydown" @focusout="onFocusout">
    <!--
      A button rather than a div: focusable, announced as a control, and the
      owner of the listbox state. Real focus stays here the whole time.
    -->
    <button
      type="button"
      class="rl-option-select__trigger"
      :class="`rl-option-select__trigger--${variant}`"
      role="combobox"
      :aria-label="label"
      :aria-expanded="open"
      aria-haspopup="listbox"
      :aria-controls="open ? `${uid}-list` : undefined"
      :aria-activedescendant="open ? optionId(activeIndex) : undefined"
      :disabled="disabled"
      @click="open ? hide() : show()"
    >
      <span class="rl-option-select__value">
        <slot v-if="modelValue" name="option" :value="modelValue" :active="false" :selected="true" />
        <span v-else class="rl-option-select__placeholder">{{ placeholder }}</span>
      </span>
      <RlIcon name="chevron-down" :size="14" class="rl-option-select__arrow" aria-hidden="true" />
    </button>

    <!-- Teleported so a scrolling table or a panel cannot clip the list. -->
    <Teleport to="body">
      <div
        v-if="open"
        :id="`${uid}-list`"
        ref="panel"
        class="rl-panel rl-panel--options rl-option-select__list"
        :style="floatingStyles"
        role="listbox"
        :aria-label="label"
        v-bind="panelHandlers"
      >
        <!--
          Options are divs, not buttons: focus never leaves the trigger, so a
          focusable option would only add a tab stop that goes nowhere.

          Which is what `keepFocusOnTrigger` above answers for: it is the
          panel's half of that decision.
        -->
        <div
          v-for="(option, index) in options"
          :id="optionId(index)"
          :key="option"
          class="rl-option-select__option"
          :class="{
            'rl-option-select__option--active': index === activeIndex && !isOptionDisabled(option),
            'rl-option-select__option--selected': option === modelValue,
            'rl-option-select__option--disabled': isOptionDisabled(option),
          }"
          role="option"
          :aria-selected="option === modelValue"
          :aria-disabled="isOptionDisabled(option) || undefined"
          @click="choose(index)"
          @mousemove="isOptionDisabled(option) || (activeIndex = index)"
        >
          <slot
            name="option"
            :value="option"
            :active="index === activeIndex"
            :selected="option === modelValue"
            :disabled="isOptionDisabled(option)"
          />
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.rl-option-select {
  display: inline-flex;
  flex: none;
  width: max-content;
  max-width: 100%;
}

/*
 * Inline pulls its own padding back so the value sits on the same left edge
 * as a static one. The offset lives here and not on the trigger: a negative
 * margin shrinks what an element contributes to its parent's intrinsic
 * width, which would size the button narrower than the text inside it.
 */
.rl-option-select:has(.rl-option-select__trigger--inline) {
  margin: calc(-1 * var(--rl-space-1)) calc(-1 * var(--rl-space-2));
}

.rl-option-select__trigger {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  max-width: 100%;
  padding: var(--rl-space-1) var(--rl-space-2);
  border: 1px solid var(--rl-color-border-strong);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg);
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

/*
 * Inline: the value, with the chrome held back until it is wanted. The
 * border is transparent rather than absent so the box does not change size
 * when it appears, which would shift the text as the pointer arrives.
 */
.rl-option-select__trigger--inline {
  border-color: transparent;
  background: transparent;
}

.rl-option-select__trigger--inline:hover:not(:disabled),
.rl-option-select__trigger--inline[aria-expanded='true'] {
  border-color: var(--rl-color-border);
  background: var(--rl-color-bg-hover);
}

/* The arrow is part of the chrome, so it keeps the same company. */
.rl-option-select__trigger--inline .rl-option-select__arrow {
  opacity: 0;
  transition: opacity var(--rl-duration-fast) var(--rl-ease);
}

.rl-option-select__trigger--inline:hover .rl-option-select__arrow,
.rl-option-select__trigger--inline:focus-visible .rl-option-select__arrow,
.rl-option-select__trigger--inline[aria-expanded='true'] .rl-option-select__arrow {
  opacity: 1;
}

@media (prefers-reduced-motion: reduce) {
  .rl-option-select__trigger--inline .rl-option-select__arrow { transition: none; }
}

.rl-option-select__trigger:disabled {
  background: var(--rl-color-bg-hover);
  cursor: not-allowed;
}

.rl-option-select__trigger:focus-visible {
  outline: none;
  border-color: var(--rl-color-focus);
  box-shadow:
    0 0 0 var(--rl-focus-ring-gap) var(--rl-color-bg),
    0 0 0 calc(var(--rl-focus-ring-gap) + var(--rl-focus-ring-width)) var(--rl-color-focus);
}

.rl-option-select__value {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
}

.rl-option-select__placeholder { color: var(--rl-color-text-subtle); }

.rl-option-select__arrow { flex: none; color: var(--rl-color-text-muted); }

/* The panel and its rows are teleported, so they are styled in panel.css
   where a scoped rule could not reach them. */
</style>
