<script setup lang="ts">
/**
 * A set of mutually exclusive choices, each able to carry a description.
 *
 * The middle of three controls for one decision, and the reason it exists:
 * `RlSegmentedControl` fits two to four options of a word each, `RlSelect`
 * fits a long list the user already knows the shape of, and neither can show
 * a line of explanation under a choice. Three to six options that need
 * explaining is the ordinary settings-page shape, and it had nothing.
 *
 * A native `radiogroup` of native radios, so arrow keys move between the
 * options and Tab enters and leaves the group as one stop, which is what a
 * reader expects and what a listbox rebuilt from divs usually gets wrong.
 *
 * The whole row is the target, description included: a choice explained by a
 * sentence the user cannot click is a smaller target than it looks.
 */
import { useId } from 'vue'
import RlFieldShell from './RlFieldShell.vue'
import RlText from '../common/RlText.vue'
import type { SelectOption } from './types'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    label: string
    options: SelectOption[]
    hint?: string
    error?: string
    required?: boolean
    disabled?: boolean
    labelHidden?: boolean
    /**
     * `card` draws each option in its own bordered box, for a choice that
     * carries a description and deserves the weight. `plain` is a bare list,
     * for short labels where the boxes would be louder than the question.
     */
    variant?: 'plain' | 'card'
    /**
     * Lays the options in a row. For two or three short labels only: a row
     * of described options wraps into something no one can scan.
     */
    inline?: boolean
  }>(),
  {
    modelValue: '',
    required: false,
    disabled: false,
    labelHidden: false,
    variant: 'plain',
    inline: false,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

/*
 * One name for the group, so the browser treats the radios as one control:
 * without it each input is its own group and every option can be chosen at
 * once.
 */
const name = useId()

function optionId(value: string) {
  return `${name}-${value}`
}

function isDisabled(option: SelectOption) {
  return props.disabled || option.disabled === true
}
</script>

<template>
  <RlFieldShell
    :label="label"
    :hint="hint"
    :error="error"
    :required="required"
    :label-hidden="labelHidden"
  >
    <template #default="{ describedBy, invalid }">
      <!--
        The group carries the invalid state and the description, not each
        radio: the error is about the choice, and repeating it on every
        option would have a reader hear it once per option.
      -->
      <div
        class="rl-radio-group"
        :class="[
          `rl-radio-group--${variant}`,
          { 'rl-radio-group--inline': inline },
        ]"
        role="radiogroup"
        :aria-describedby="describedBy"
        :aria-invalid="invalid || undefined"
        :aria-required="required || undefined"
      >
        <label
          v-for="option in options"
          :key="option.value"
          class="rl-radio"
          :class="{ 'rl-radio--disabled': isDisabled(option) }"
        >
          <input
            :id="optionId(option.value)"
            type="radio"
            class="rl-radio__input"
            :name="name"
            :value="option.value"
            :checked="modelValue === option.value"
            :disabled="isDisabled(option)"
            :aria-describedby="option.description ? `${optionId(option.value)}-desc` : undefined"
            @change="emit('update:modelValue', option.value)"
          />

          <span class="rl-radio__marker" aria-hidden="true" />

          <span class="rl-radio__text">
            <span class="rl-radio__label">{{ option.label }}</span>
            <RlText
              v-if="option.description"
              :id="`${optionId(option.value)}-desc`"
              as="span"
              size="sm"
              tone="subtle"
              class="rl-radio__description"
            >
              {{ option.description }}
            </RlText>
          </span>
        </label>
      </div>
    </template>
  </RlFieldShell>
</template>

<style scoped>
.rl-radio-group {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.rl-radio-group--inline {
  flex-direction: row;
  flex-wrap: wrap;
  gap: var(--rl-space-4);
}

.rl-radio {
  display: flex;
  align-items: flex-start;
  gap: var(--rl-space-3);
  cursor: pointer;
}

/*
 * The real input keeps focus, the keyboard and forced-colours rendering; it
 * is only made invisible, with the marker drawn beside it. `appearance: none`
 * would style the real control but loses the high-contrast rendering, which
 * is the one mode where the marker has to be right.
 */
.rl-radio__input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: 0;
  padding: 0;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}

.rl-radio__marker {
  position: relative;
  flex: none;
  width: 16px;
  height: 16px;
  /* Aligns the circle to the cap height of the label beside it. */
  margin-top: 2px;
  border: 1px solid var(--rl-color-border-strong);
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-bg);
}

.rl-radio__input:checked + .rl-radio__marker {
  border-color: var(--rl-color-accent);
  background: var(--rl-color-accent);
}

/* The dot is drawn in the knocked-out centre rather than as a second element. */
.rl-radio__input:checked + .rl-radio__marker::after {
  content: '';
  position: absolute;
  inset: 4px;
  border-radius: var(--rl-radius-pill);
  /*
   * Knocked out of the accent fill, so it is the page showing through rather
   * than a light shape drawn on top. The accent is light in both themes, so a
   * light dot on it would all but disappear.
   */
  background: var(--rl-color-bg);
}

.rl-radio__input:focus-visible + .rl-radio__marker {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 2px;
}

.rl-radio__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.rl-radio__description { display: block; }

.rl-radio--disabled {
  cursor: default;
  color: var(--rl-color-text-subtle);
}

.rl-radio--disabled .rl-radio__marker {
  border-color: var(--rl-color-border);
  background: var(--rl-color-bg-sunken);
}

/*
 * Boxed, for options carrying a description. The border makes each choice
 * one object rather than two lines of text that happen to sit together.
 */
.rl-radio-group--card .rl-radio {
  padding: var(--rl-space-3) var(--rl-space-4);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-raised);
}

.rl-radio-group--card .rl-radio:hover:not(.rl-radio--disabled) {
  border-color: var(--rl-color-border-strong);
}

/*
 * The chosen card is marked by its border and a tint rather than by the
 * marker alone: at card size a 16px circle is too small to carry the state
 * across the whole object it applies to.
 */
.rl-radio-group--card .rl-radio:has(.rl-radio__input:checked) {
  border-color: var(--rl-color-accent);
  background: var(--rl-color-bg-selected);
}

.rl-radio-group--card .rl-radio:has(.rl-radio__input:focus-visible) {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 2px;
}

/* The ring is on the card, so the marker does not draw a second one inside it. */
.rl-radio-group--card .rl-radio__input:focus-visible + .rl-radio__marker {
  outline: none;
}

@media (pointer: coarse) {
  .rl-radio-group--plain .rl-radio { padding-block: var(--rl-space-1); }
}
</style>
