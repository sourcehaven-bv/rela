<script setup lang="ts">
/**
 * Click-to-edit: a value that reads as text until you click it, then becomes
 * a control in the same spot.
 *
 * Both halves are slots, so this holds no opinion about what is being edited.
 * The read half is whatever shows the value — a status dot and a label, a
 * tag, a date, plain text. The edit half is any control that takes focus and
 * reports a value. That is the whole point: status is not a special case
 * here, it is one pairing of a read view with a control.
 *
 * What the component owns is the part that is easy to get wrong: swapping the
 * two halves, moving focus into the control and back to the trigger
 * afterwards, and deciding when an edit is kept. Enter and a blur out of the
 * control commit; Escape cancels and restores the value that was there when
 * editing started.
 *
 * # When the read view is not plain text
 *
 * The read half is a button by default, which is right for a value: one
 * target, reachable by Tab, announced as an action. It is wrong for rendered
 * content, because a button cannot hold a link, a checkbox or anything else
 * the user is meant to click, and nesting one inside it is invalid HTML.
 *
 * `trigger="explicit"` is for that case. See the prop.
 */
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import RlIcon from './RlIcon.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    /**
     * Names the value for assistive tech. The read button announces as
     * "Owner, Rita Bloem, edit", so the label is not repeated on screen.
     */
    label: string
    /** Shown in the read view when there is nothing to show. */
    placeholder?: string
    /** Renders the value as static text, with no affordance to edit it. */
    disabled?: boolean
    /**
     * Commits as soon as a native control reports a change, instead of
     * waiting for Enter or blur. Right for a plain `select` or checkbox,
     * where the choice is the whole edit and confirming it again is friction.
     *
     * Only native `change` events reach this. A control that is a component
     * emits its own events, so it should call the slot's `commit` instead.
     */
    commitOnChange?: boolean
    /**
     * Whether the read view has a value. Used only to pick between the value
     * and the placeholder, because the value itself lives in the slot.
     */
    empty?: boolean
    /**
     * Opens the control's own picker as soon as it appears, so a date takes
     * one click rather than two: one to reveal the input and another to open
     * the calendar.
     *
     * Only for a control that has a picker to open. It is not the default
     * because for a text box there is nothing to open, and for a `select` it
     * would mean a list appearing without being asked for.
     */
    autoOpenPicker?: boolean
    /**
     * Lays the value out as a block that fills its container, rather than
     * hugging its own text. For prose and anything else whose width is the
     * column it sits in: sizing those to their content makes the box jump
     * between the read and edit views.
     */
    block?: boolean
    /**
     * What starts the edit.
     *
     * `value` wraps the read view in a button: the whole value is the target.
     * Right for a value, which has nothing inside it to click.
     *
     * `explicit` renders the read view in a plain div and puts a small edit
     * button beside it, revealed on hover and focus-within but always in the
     * tab order. A click on the content still starts the edit, except where
     * it lands on something interactive of the caller's own, or where it ends
     * a text selection — a drag to select is not a request to edit.
     *
     * For rendered content: links, task checkboxes, comment markers, a
     * diagram. Mark anything else the click should leave alone with
     * `data-rl-inline-edit-ignore`.
     */
    trigger?: 'value' | 'explicit'
    /**
     * Extra elements that count as inside the editor for the purpose of
     * leaving it. A control with a floating toolbar or a mention menu renders
     * those outside its own element, often at the end of the body, so focus
     * moving into one looks like focus leaving the edit and would commit in
     * the middle of using the toolbar.
     *
     * Called each time the edit might be ending, so it can return whatever is
     * mounted at that moment.
     */
    keepOpenWithin?: () => Element[]
  }>(),
  {
    placeholder: 'Empty',
    disabled: false,
    commitOnChange: false,
    empty: false,
    autoOpenPicker: false,
    block: false,
    trigger: 'value',
    keepOpenWithin: () => [],
  },
)

const emit = defineEmits<{
  /** The user kept the edit. */
  commit: []
  /** The user backed out. Restore whatever the value was before. */
  cancel: []
  edit: []
}>()

const editing = ref(false)
const triggerEl = ref<HTMLElement | null>(null)
const editor = ref<HTMLElement | null>(null)

/**
 * Focus the first thing in the edit slot. The caller supplies the control, so
 * the component cannot hold a ref to it directly and looks it up instead.
 */
function focusControl() {
  const control = editor.value?.querySelector<HTMLElement>(
    'input, select, textarea, button, [tabindex]:not([tabindex="-1"])',
  )
  control?.focus()

  if (props.autoOpenPicker && control instanceof HTMLInputElement) {
    /*
     * `showPicker` needs a user gesture, and the click that started the edit
     * is one. It throws where that does not hold, or where the browser has no
     * picker for this input type, so a failure just leaves the field focused
     * and typeable — which is the behaviour without it anyway.
     */
    try {
      control.showPicker()
    } catch {
      // No picker available. The input is focused, which is enough.
    }
    return
  }

  if (control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement) {
    control.select()
  }
}

async function start() {
  if (props.disabled || editing.value) return
  editing.value = true
  emit('edit')
  await nextTick()
  focusControl()
}

/**
 * Leaving edit mode always returns focus to the trigger, so a keyboard user
 * ends where they started rather than at the top of the document.
 */
async function stop(outcome: 'commit' | 'cancel') {
  if (!editing.value) return
  editing.value = false
  // Narrowed, because the emit overloads accept one literal event name each.
  if (outcome === 'commit') emit('commit')
  else emit('cancel')
  await nextTick()
  triggerEl.value?.focus()
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    // Stopped so an inline edit inside a modal does not close the modal too.
    event.stopPropagation()
    stop('cancel')
  } else if (event.key === 'Enter' && !(event.target instanceof HTMLTextAreaElement)) {
    // A textarea keeps Enter for newlines; everything else treats it as done.
    event.preventDefault()
    stop('commit')
  }
}

/**
 * Whether a node is part of the edit: the editor itself, or one of the
 * panels the caller has named as belonging to it.
 */
function isInside(node: Node | null) {
  if (!node) return false
  if (editor.value?.contains(node)) return true
  return props.keepOpenWithin().some((element) => element.contains(node))
}

/**
 * Commit when focus leaves the editor for good. A move between two elements
 * inside it — a select and a clear button, say — is not leaving.
 */
function onFocusout(event: FocusEvent) {
  if (isInside(event.relatedTarget as Node | null)) return
  stop('commit')
}

/*
 * Focus alone cannot say when an edit with a floating panel is over.
 *
 * This handler is bound on the editor, so once focus has moved into a panel
 * rendered at the end of the body, every later focusout happens somewhere
 * this element never hears about. And a press on something non-focusable
 * moves no focus at all, so it fires nothing anywhere.
 *
 * A press outside every part of the edit answers both: it is the moment the
 * user turned their attention elsewhere, whether or not anything took focus.
 * `pointerdown` rather than `click`, so the commit lands before the press
 * completes and the click still reaches whatever was pressed — leaving an
 * edit should not cost the user the click that left it.
 */
function onDocumentPointerdown(event: PointerEvent) {
  if (isInside(event.target as Node | null)) return
  stop('commit')
}

watch(editing, (active) => {
  if (active) document.addEventListener('pointerdown', onDocumentPointerdown, true)
  else document.removeEventListener('pointerdown', onDocumentPointerdown, true)
})

// Unmounting mid-edit would otherwise leave the listener behind.
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocumentPointerdown, true)
})

function onChange() {
  if (props.commitOnChange) stop('commit')
}

/** What a click must not turn into an edit, because it is already something. */
const INTERACTIVE =
  'a, button, input, select, textarea, label, summary, [role="button"], [role="link"], [role="checkbox"], [contenteditable="true"], [data-rl-inline-edit-ignore]'

/**
 * A click on the read content, when the content is the caller's own. It
 * starts the edit only if it was not aimed at something else.
 */
function onReadClick(event: MouseEvent) {
  /*
   * The content already did something with this click, so it was not a
   * request to edit. This is bound on the wrapper and the content sits
   * inside it, so a handler of the caller's has already run by now.
   *
   * It is the general form of the check below: a delegated handler on a
   * comment highlight or a diagram node cannot be named by a selector, and
   * should not have to be marked by hand.
   */
  if (event.defaultPrevented) return

  const target = event.target as Element | null
  if (target?.closest(INTERACTIVE)) return
  // A drag that ends here selected text; the user was reading, not editing.
  if (!window.getSelection()?.isCollapsed) return
  start()
}

defineExpose({ start, cancel: () => stop('cancel') })
</script>

<template>
  <div
    class="rl-inline-edit"
    :class="{ 'rl-inline-edit--editing': editing, 'rl-inline-edit--block': block }"
  >
    <!--
      A real button, not a clickable div: it is already reachable by Tab,
      activates on Enter and Space, and announces itself as an action.
    -->
    <button
      v-if="!editing && !disabled && trigger !== 'explicit'"
      ref="triggerEl"
      type="button"
      class="rl-inline-edit__trigger"
      :class="{ 'rl-inline-edit__trigger--empty': empty }"
      :aria-label="messages.editField({ label })"
      @click="start"
    >
      <span class="rl-inline-edit__value">
        <slot v-if="!empty" name="read" />
        <span v-else class="rl-inline-edit__placeholder">{{ placeholder }}</span>
      </span>
    </button>

    <!--
      Explicit: the content is the caller's, so it is rendered plainly and the
      click is only interpreted. The edit button is the reliable way in, and
      is a real button in the tab order rather than a hover affordance, since
      a control that only appears on hover cannot be reached without a mouse.
    -->
    <div
      v-else-if="!editing && !disabled"
      class="rl-inline-edit__trigger rl-inline-edit__read"
      :class="{ 'rl-inline-edit__trigger--empty': empty }"
      @click="onReadClick"
    >
      <span class="rl-inline-edit__value">
        <slot v-if="!empty" name="read" />
        <span v-else class="rl-inline-edit__placeholder">{{ placeholder }}</span>
      </span>

      <button
        ref="triggerEl"
        type="button"
        class="rl-inline-edit__edit-button"
        :aria-label="messages.editField({ label })"
        @click.stop="start"
      >
        <RlIcon name="edit" :size="14" />
      </button>
    </div>

    <!-- Disabled reads as the value alone: an affordance that does nothing is
         worse than none at all. -->
    <span v-else-if="!editing" class="rl-inline-edit__static">
      <slot v-if="!empty" name="read" />
      <span v-else class="rl-inline-edit__placeholder">{{ placeholder }}</span>
    </span>

    <div
      v-else
      ref="editor"
      class="rl-inline-edit__editor"
      @keydown="onKeydown"
      @focusout="onFocusout"
      @change="onChange"
    >
      <slot name="edit" :commit="() => stop('commit')" :cancel="() => stop('cancel')" />
    </div>
  </div>
</template>

<style scoped>
/*
 * Sized to its own content, padding included.
 *
 * These usually sit inside a flex container — a detail row, a table cell —
 * which blockifies `inline-flex` to `flex` and sizes them by the flex
 * algorithm rather than by what is inside. The result is a box narrower than
 * its own text: the background stops while the glyphs keep going.
 *
 * `flex: none` opts out of that sizing. The width must be `max-content` and
 * not `fit-content`: `fit-content` clamps to the space available, which the
 * trigger's negative margin has already shrunk, so it reproduces the bug it
 * was meant to fix.
 */
.rl-inline-edit {
  display: inline-flex;
  flex: none;
  width: max-content;
  max-width: 100%;
}

/*
 * The background hugs the value, with padding to lift it off the text.
 *
 * The padding is not cancelled by a negative margin, tempting as that is for
 * keeping the text on the same left edge as a non-editable value: a negative
 * margin shrinks what the element contributes to its parent's intrinsic
 * width, so `max-content` then resolves to less than the content needs and
 * the background stops short of its own text. The row pays for the offset
 * instead, via the padding compensation below.
 */
.rl-inline-edit__trigger {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  flex: none;
  width: max-content;
  max-width: 100%;
  padding: var(--rl-space-1) var(--rl-space-2);
  border: 1px solid transparent;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.rl-inline-edit__trigger:hover {
  background: var(--rl-color-bg-hover);
  border-color: var(--rl-color-border);
}

.rl-inline-edit__trigger:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

/*
 * The read view lays its content out and nothing more. Whether a long value
 * wraps or truncates depends on where it sits — a table cell clips, a
 * description paragraph wraps — so that choice belongs to the caller.
 */
.rl-inline-edit__value {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-inline-edit__static {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
}

.rl-inline-edit__placeholder { color: var(--rl-color-text-subtle); }

.rl-inline-edit__editor {
  display: flex;
  align-items: center;
  min-width: 0;
}

/*
 * The trigger's padding is offset on the container, not on the trigger, so
 * the value still lines up with a non-editable one while the trigger itself
 * keeps its full intrinsic width.
 */
.rl-inline-edit {
  margin: calc(-1 * var(--rl-space-1)) calc(-1 * var(--rl-space-2));
}

/*
 * Block: the value is as wide as the column, not as wide as its text. Both
 * halves fill, so swapping one for the other does not resize the box.
 */
.rl-inline-edit--block,
.rl-inline-edit--block .rl-inline-edit__trigger,
.rl-inline-edit--block .rl-inline-edit__value,
.rl-inline-edit--block .rl-inline-edit__editor {
  display: block;
  /*
   * `100%` and not `auto`: a block-level button is shrink-to-fit, so `auto`
   * would size it to the prose and the box would resize on every swap.
   */
  width: 100%;
  max-width: none;
}

.rl-inline-edit--block { flex: 1; }

/*
 * The editor takes the trigger's padding too, so both halves put their
 * content on the same left edge and swapping does not shift the prose.
 */
.rl-inline-edit--block .rl-inline-edit__editor {
  padding: var(--rl-space-1) var(--rl-space-2);
  border: 1px solid transparent;
}

/*
 * Explicit: the same box as the button trigger, but a div, so the content
 * inside keeps its own clicks. The hover treatment is the button's, because
 * it still has to read as editable.
 */
.rl-inline-edit__read {
  position: relative;
  cursor: text;
}

.rl-inline-edit__read:hover {
  background: var(--rl-color-bg-hover);
  border-color: var(--rl-color-border);
}

/*
 * Held back until wanted, like the rest of the chrome, but present in the
 * tab order the whole time: `opacity` hides it without taking it out of the
 * document, which `display: none` would.
 */
.rl-inline-edit__edit-button {
  position: absolute;
  top: var(--rl-space-1);
  right: var(--rl-space-1);
  display: flex;
  padding: var(--rl-space-1);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg);
  color: var(--rl-color-text-muted);
  opacity: 0;
  cursor: pointer;
  transition: opacity var(--rl-duration-fast) var(--rl-ease);
}

.rl-inline-edit__read:hover .rl-inline-edit__edit-button,
.rl-inline-edit__edit-button:focus-visible {
  opacity: 1;
}

.rl-inline-edit__edit-button:hover { color: var(--rl-color-text); }

.rl-inline-edit__edit-button:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

@media (prefers-reduced-motion: reduce) {
  .rl-inline-edit__edit-button { transition: none; }
}

@media (pointer: coarse) {
  /* A tap target smaller than this is hard to hit deliberately. */
  .rl-inline-edit__trigger { min-height: var(--rl-tap-target); }

  /* No hover to reveal it, so it stays. */
  .rl-inline-edit__edit-button {
    opacity: 1;
    min-width: var(--rl-tap-target);
    min-height: var(--rl-tap-target);
    align-items: center;
    justify-content: center;
  }
}
</style>
