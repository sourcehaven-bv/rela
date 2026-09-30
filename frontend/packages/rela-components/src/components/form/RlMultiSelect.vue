<script setup lang="ts">
/**
 * Multi-choice field: the chosen values show in the trigger, and every choice
 * is ticked or unticked in a checkbox list. Unticking is how a value is
 * removed; the trigger shows what is chosen rather than offering a second way
 * to undo it.
 *
 * A native `select multiple` is skipped on purpose. It requires ctrl-clicking
 * to add without clearing, gives no indication that it is multi-choice, and
 * is close to unusable on a phone.
 *
 * What an option looks like comes from the `option` slot, in the panel and in
 * the trigger both, so a tag keeps its colour while it is being chosen and
 * after it has been. Without it the label is drawn as a plain chip.
 *
 * `inline` is the form for a value sitting in a record rather than a form,
 * matching `RlOptionSelect`: the values alone until hovered, and one click to
 * open. See the prop.
 */
import { computed, ref } from 'vue'
import { useAnchoredPanel } from '../../composables/useAnchoredPanel'
import RlFieldShell from './RlFieldShell.vue'
import RlIcon from '../common/RlIcon.vue'
import RlText from '../common/RlText.vue'
import type { SelectOption } from './types'

const props = withDefaults(
  defineProps<{
    modelValue?: string[]
    label: string
    /** Hides the label visually, keeping it for assistive technology. */
    labelHidden?: boolean
    options: SelectOption[]
    hint?: string
    error?: string
    required?: boolean
    disabled?: boolean
    placeholder?: string
    /**
     * Whether a single option cannot be chosen, on top of the option's own
     * `disabled`. For a rule that is not a property of the choice itself —
     * a permission, say — and so cannot be baked into the list.
     *
     * A disabled option that is already chosen still shows in the trigger,
     * and its checkbox is disabled while staying ticked: a rule that forbids
     * picking something forbids unpicking it too.
     */
    isOptionDisabled?: (value: string) => boolean
    /**
     * `inline` renders the trigger bare, as the chosen values alone, and only
     * draws the button chrome on hover and focus. For a value in a detail row
     * or a table cell, where a permanent box reads as a form rather than as a
     * record.
     *
     * It drops the field shell with it: the label, the hint and the error are
     * a form's furniture, and inline the row already says what the field is.
     * `label` stays, for assistive tech.
     */
    variant?: 'control' | 'inline'
  }>(),
  {
    modelValue: () => [],
    required: false,
    labelHidden: false,
    disabled: false,
    placeholder: 'Select options',
    isOptionDisabled: () => false,
    variant: 'control',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
  /**
   * The panel closed. Every toggle has already been emitted through
   * `update:modelValue`; this marks the end of a run of them, for a caller
   * that would rather save once per visit than once per checkbox.
   */
  close: []
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)

/*
 * `matchWidth` keeps the list the same width as the control: a panel that is
 * wider or narrower reads as a separate thing rather than as part of the
 * field. It opens downward but flips up near the foot of the page.
 *
 * Not inline, though. There the trigger hugs the values it holds, so matching
 * it would size the list by how much happens to be chosen and squeeze the
 * labels being chosen between.
 */
const { floatingStyles, containsTarget, panelHandlers } = useAnchoredPanel(root, panel, {
  placement: 'bottom-start',
  // Focus stays on the trigger: a row is a label wrapping its own checkbox,
  // so a press must not move focus off the trigger and tear the panel down.
  keepFocusOnTrigger: true,
  // A plain read, not a computed: the composable takes this once, and a
  // variant does not change over a component's life.
  matchWidth: props.variant === 'control',
})

const selected = computed(() =>
  props.modelValue
    .map((value) => props.options.find((option) => option.value === value))
    .filter((option): option is SelectOption => Boolean(option)),
)

/** Forbidden either by the option itself or by a rule outside the list. */
const isDisabled = (option: SelectOption) =>
  Boolean(option.disabled) || props.isOptionDisabled(option.value)

function toggle(value: string) {
  const option = props.options.find((entry) => entry.value === value)
  if (option && isDisabled(option)) return
  const next = props.modelValue.includes(value)
    ? props.modelValue.filter((entry) => entry !== value)
    : [...props.modelValue, value]
  emit('update:modelValue', next)
}

/*
 * Every way the panel closes goes through here, so a caller listening for
 * `close` hears the end of an edit however it ended.
 */
function hide() {
  if (!open.value) return
  open.value = false
  emit('close')
}

/**
 * Closing on focusout rather than on a document click keeps the keyboard and
 * the pointer in step: tabbing past the last checkbox closes the list too.
 */
function onFocusout(event: FocusEvent) {
  // The panel is teleported out of `root`, so the composable answers for both.
  if (!containsTarget(root.value, event.relatedTarget as Node | null)) hide()
}
</script>

<template>
  <!--
    Inline drops the shell entirely rather than hiding it: the label, the hint
    and the error are a form's furniture, and the row an inline value sits in
    has already said what the field is.
  -->
  <RlFieldShell
    v-if="variant === 'control'"
    v-slot="control"
    :label="label"
    :label-hidden="labelHidden"
    :hint="hint"
    :error="error"
    :required="required"
  >
    <div ref="root" class="rl-multi" @focusout="onFocusout" @keydown.escape="hide">
      <button
        :id="control.id"
        type="button"
        class="rl-control rl-multi__trigger"
        :disabled="disabled"
        :aria-expanded="open"
        aria-haspopup="true"
        :aria-describedby="control.describedBy"
        :aria-invalid="control.invalid || undefined"
        @click="open ? hide() : (open = true)"
      >
        <span class="rl-multi__value">
          <template v-if="selected.length">
            <span v-for="option in selected" :key="option.value" class="rl-multi__selected">
              <slot name="option" :value="option.value" :option="option" :selected="true" :disabled="isDisabled(option)">
                <span class="rl-multi__chip">{{ option.label }}</span>
              </slot>
            </span>
          </template>
          <RlText v-else size="md" tone="subtle">{{ placeholder }}</RlText>
        </span>
        <RlIcon name="chevron-down" :size="16" class="rl-multi__arrow" aria-hidden="true" />
      </button>
    </div>
  </RlFieldShell>

  <div
    v-else
    ref="root"
    class="rl-multi rl-multi--inline"
    @focusout="onFocusout"
    @keydown.escape="hide"
  >
    <button
      type="button"
      class="rl-multi__trigger rl-multi__trigger--inline"
      :aria-label="label"
      :disabled="disabled"
      :aria-expanded="open"
      aria-haspopup="true"
      @click="open ? hide() : (open = true)"
    >
      <span class="rl-multi__value">
        <template v-if="selected.length">
          <span v-for="option in selected" :key="option.value" class="rl-multi__selected">
            <slot name="option" :value="option.value" :option="option" :selected="true" :disabled="isDisabled(option)">
              <span class="rl-multi__chip">{{ option.label }}</span>
            </slot>
          </span>
        </template>
        <RlText v-else size="md" tone="subtle">{{ placeholder }}</RlText>
      </span>
      <RlIcon name="chevron-down" :size="14" class="rl-multi__arrow" aria-hidden="true" />
    </button>
  </div>

  <!--
    A group rather than a listbox: the options are real checkboxes, so
    they already announce their own checked state and need no extra roles.
    Teleported so a scrolling form or a modal body cannot clip it.

    One panel for both variants, so the two cannot drift apart.

    `keepFocusOnTrigger` is what keeps a press on a row's drawn label — a
    tag, a badge — from moving focus and tearing the panel down before the
    click reaches the checkbox. The checkbox is still reached by click and by
    keyboard; only the focus move is suppressed.
  -->
  <Teleport to="body">
    <div
      v-if="open"
      ref="panel"
      class="rl-panel rl-panel--options"
      :style="floatingStyles"
      role="group"
      :aria-label="label"
      @focusout="onFocusout"
      @keydown.escape="hide"
      v-bind="panelHandlers"
    >
      <label
        v-for="option in options"
        :key="option.value"
        class="rl-multi__option"
        :class="{ 'rl-multi__option--disabled': isDisabled(option) }"
      >
        <input
          type="checkbox"
          :checked="modelValue.includes(option.value)"
          :disabled="isDisabled(option)"
          @change="toggle(option.value)"
        />
        <slot name="option" :value="option.value" :option="option" :selected="modelValue.includes(option.value)" :disabled="isDisabled(option)">
          {{ option.label }}
        </slot>
      </label>
    </div>
  </Teleport>
</template>

<style scoped>
.rl-multi { position: relative; }

.rl-multi__trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--rl-space-2);
  padding-top: var(--rl-space-1);
  padding-bottom: var(--rl-space-1);
  text-align: left;
  cursor: pointer;
}

.rl-multi__value {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-1);
  min-width: 0;
}

.rl-multi__chip {
  padding: 1px var(--rl-space-2);
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-bg-hover);
  font-size: var(--rl-font-size-sm);
}

.rl-multi__selected { display: inline-flex; min-width: 0; }

/*
 * Inline sizes itself to the values, so they must not be shrunk to fit a
 * width that is derived from them in the first place. `min-width: 0` on the
 * value row is what a fixed-width field needs to keep its chips inside its
 * box; here it only squeezes each tag below its own text.
 */
.rl-multi--inline .rl-multi__value { min-width: max-content; }

.rl-multi--inline .rl-multi__selected { flex: none; min-width: max-content; }

.rl-multi__arrow { flex: none; color: var(--rl-color-text-muted); }

/*
 * Inline: the values, with the chrome held back until wanted. Mirrors
 * `RlOptionSelect`, since the two sit in the same kind of row and a value
 * should not change shape depending on how many of it there can be.
 */
.rl-multi--inline {
  display: inline-flex;
  flex: none;
  width: max-content;
  max-width: 100%;
  /*
   * The vertical offset only: the padding keeps the row from growing taller,
   * and no width is derived from it.
   *
   * Not the horizontal one. A negative margin shrinks what an element
   * contributes to its parent's intrinsic width, so `max-content` on the
   * button below then resolves to less than its own contents need and the
   * chevron ends up outside the background. An inline value here therefore
   * sits one step further right than a static one, which is the cheaper of
   * the two wrongs.
   */
  margin: calc(-1 * var(--rl-space-1)) 0;
}

.rl-multi__trigger--inline {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  /*
   * Sized to its own contents, chrome included. The button is a flex child,
   * which blockifies `inline-flex` and hands its width to the flex algorithm
   * instead; the background then stops short of the arrow, which keeps its
   * place and sits outside the box. `flex: none` opts out of that sizing,
   * and `max-content` is what the contents actually need.
   */
  flex: none;
  width: max-content;
  max-width: 100%;
  padding: var(--rl-space-1) var(--rl-space-2);
  /* Transparent rather than absent, so the box does not change size when the
     border appears and shift the values under the pointer. */
  border: 1px solid transparent;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.rl-multi__trigger--inline:hover:not(:disabled),
.rl-multi__trigger--inline[aria-expanded='true'] {
  border-color: var(--rl-color-border);
  background: var(--rl-color-bg-hover);
}

.rl-multi__trigger--inline:disabled { cursor: not-allowed; }

.rl-multi__trigger--inline:focus-visible {
  outline: none;
  border-color: var(--rl-color-focus);
  box-shadow:
    0 0 0 var(--rl-focus-ring-gap) var(--rl-color-bg),
    0 0 0 calc(var(--rl-focus-ring-gap) + var(--rl-focus-ring-width)) var(--rl-color-focus);
}

/* The arrow is part of the chrome, so it keeps the same company. */
.rl-multi__trigger--inline .rl-multi__arrow {
  opacity: 0;
  transition: opacity var(--rl-duration-fast) var(--rl-ease);
}

.rl-multi__trigger--inline:hover .rl-multi__arrow,
.rl-multi__trigger--inline:focus-visible .rl-multi__arrow,
.rl-multi__trigger--inline[aria-expanded='true'] .rl-multi__arrow {
  opacity: 1;
}

@media (prefers-reduced-motion: reduce) {
  .rl-multi__trigger--inline .rl-multi__arrow { transition: none; }
}

/* The panel and its option rows are styled in styles/panel.css: they are
   teleported, so a scoped rule here would not reach them. */

</style>
