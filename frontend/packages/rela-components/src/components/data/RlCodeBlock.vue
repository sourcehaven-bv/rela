<script setup lang="ts">
/**
 * A block of code, with a way to copy it.
 *
 * For showing code rather than editing it: a Lua snippet, a YAML metamodel
 * fragment, a curl command, an error's stack. `RlMarkdownEditor` renders fenced
 * code inside a document; this is the standalone block, which is the shape a
 * help panel, a settings page or an error dialog needs.
 *
 * ## No syntax highlighting
 *
 * Highlighting means a grammar per language and a tokeniser to run them, which
 * is 100kB of dependency for every consumer whether or not they show code. It
 * is also the part an app is most likely to already own, since the same
 * highlighter usually serves its docs.
 *
 * So the code is a slot as well as a prop. Pass `code` for plain text and this
 * component handles the copy, the scroll and the line numbers; put highlighted
 * markup in the default slot and it keeps all of that and stops styling the
 * text. `language` is recorded either way, both as a label and as the
 * `language-*` class a highlighter looks for.
 *
 * ## Copying is the point
 *
 * A code block nobody can copy from is a picture of code. The button reports
 * success in place rather than through a toast: the acknowledgement belongs
 * beside the thing that was copied, and a block in a modal may have no toast
 * host above it.
 */
import { computed, ref, onBeforeUnmount } from 'vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlText from '../common/RlText.vue'

const props = withDefaults(
  defineProps<{
    /**
     * The code, as text. Leave it off and fill the default slot instead when
     * the caller has already highlighted it.
     *
     * Still worth passing alongside a highlighted slot: it is what the copy
     * button puts on the clipboard, and copying the rendered markup's
     * `textContent` instead would carry the line numbers with it.
     */
    code?: string
    /**
     * The language, for the label and for the `language-<name>` class that
     * every highlighter keys off. Not validated against a list: the set of
     * languages is the app's business, and a name this component rejected
     * would simply be unlabelled.
     */
    language?: string
    /** A caption above the block, such as a file path. */
    title?: string
    /**
     * Whether to number the lines. Off by default: numbers earn their place
     * when something refers to a line, and otherwise they are furniture that
     * also has to be excluded from a copy.
     */
    lineNumbers?: boolean
    /**
     * Rows past which the block scrolls instead of growing. Zero lets it grow
     * to whatever it holds.
     *
     * Counted in lines rather than pixels so it stays right when a consumer
     * changes the mono font's size.
     */
    maxLines?: number
    /** Whether to offer the copy button. */
    copyable?: boolean
    /** Label for the copy control, and its confirmation. */
    copyLabel?: string
    copiedLabel?: string
  }>(),
  {
    code: undefined,
    language: undefined,
    title: undefined,
    lineNumbers: false,
    maxLines: 0,
    copyable: true,
    copyLabel: 'Copy code',
    copiedLabel: 'Copied',
  },
)

const emit = defineEmits<{
  /**
   * The code was copied. Carries nothing: the caller already has the code, and
   * what it wants to know is that a copy happened, usually to count it.
   */
  copy: []
  /**
   * Copying failed, with the reason. Worth handling: the clipboard needs a
   * secure context and a recent user gesture, so a block inside a long-running
   * panel can fail for reasons the user cannot see.
   */
  copyError: [error: unknown]
}>()

const lines = computed(() => (props.code ?? '').replace(/\n$/, '').split('\n'))

/*
 * The scroll cap as a line count. `em` rather than the line-height token's
 * unitless value multiplied out, so the two cannot drift.
 */
const maxHeight = computed(() =>
  props.maxLines > 0
    ? `calc(${props.maxLines} * ${'1em'} * var(--rl-line-height-normal))`
    : undefined,
)

const copied = ref(false)
let resetTimer: ReturnType<typeof setTimeout> | undefined

/*
 * Cleared on unmount: a block copied and then closed — a modal, a dismissed
 * panel — would otherwise leave a timer writing to a gone component.
 */
onBeforeUnmount(() => clearTimeout(resetTimer))

async function copy() {
  /*
   * The prop rather than the rendered text. A highlighted slot renders spans,
   * and with line numbers on, reading `textContent` back would put "1" at the
   * start of the first line.
   */
  const text = props.code
  if (text === undefined) return

  try {
    /*
     * `navigator.clipboard` is absent, not just refusing, outside a secure
     * context — an app served over plain HTTP on a LAN, or an embedded
     * webview. Reaching straight for `writeText` there throws a TypeError
     * rather than rejecting, which would escape this `catch` in a way the
     * `copy-error` event is meant to prevent.
     */
    if (!navigator.clipboard?.writeText) {
      throw new Error('The clipboard is unavailable outside a secure context.')
    }
    await navigator.clipboard.writeText(text)
    copied.value = true
    emit('copy')
    clearTimeout(resetTimer)
    resetTimer = setTimeout(() => (copied.value = false), 2000)
  } catch (error) {
    /*
     * Reported rather than swallowed, and the button does not claim success.
     * A silent failure is the worst outcome here: the user moves on believing
     * they have the code.
     */
    emit('copyError', error)
  }
}
</script>

<template>
  <div class="rl-code-block">
    <div v-if="title || language || copyable" class="rl-code-block__header">
      <RlText v-if="title" size="sm" tone="muted" class="rl-code-block__title">{{ title }}</RlText>
      <!--
        The language reads as a quiet label rather than a tag: it describes the
        block instead of categorising it, and a tag here would compete with the
        code for the eye.
      -->
      <span v-if="language && !title" class="rl-code-block__language">{{ language }}</span>

      <div class="rl-code-block__actions">
        <!--
          The confirmation is a live region beside the button, not a toast: the
          acknowledgement belongs next to what was copied, and a block inside a
          modal may have no toast host above it.
        -->
        <span
          v-if="copied"
          class="rl-code-block__copied"
          role="status"
          aria-live="polite"
          >{{ copiedLabel }}</span
        >
        <RlIconButton
          v-if="copyable && code !== undefined"
          :icon="copied ? 'check' : 'copy'"
          :label="copyLabel"
          @click="copy"
        />
      </div>
    </div>

    <!--
      `tabindex="0"` so a block that scrolls can be reached by keyboard: a
      scrollable region with no focusable child is otherwise unreachable
      without a pointer. Only when it actually scrolls, since a needless tab
      stop on every block is its own problem.
    -->
    <pre
      class="rl-code-block__pre"
      :class="{ 'rl-code-block__pre--numbered': lineNumbers }"
      :style="maxHeight ? { maxHeight } : undefined"
      :tabindex="maxLines > 0 ? 0 : undefined"
      :role="maxLines > 0 ? 'group' : undefined"
      :aria-label="maxLines > 0 ? (title ?? language ?? 'Code') : undefined"
    ><code :class="language ? `language-${language}` : undefined"><slot><template v-if="lineNumbers"><span v-for="(line, index) in lines" :key="index" class="rl-code-block__line"><span class="rl-code-block__number" aria-hidden="true">{{ index + 1 }}</span><span class="rl-code-block__text">{{ line }}</span></span></template><template v-else>{{ code }}</template></slot></code></pre>
  </div>
</template>

<style scoped>
.rl-code-block {
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-sunken);
  overflow: hidden;
}

.rl-code-block__header {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-1) var(--rl-space-1) var(--rl-space-1) var(--rl-space-3);
  border-bottom: 1px solid var(--rl-color-border);
  /*
   * A shade off the code surface, so the header reads as chrome. Sunken on
   * sunken would leave the copy button floating in the code.
   */
  background: var(--rl-color-bg);
}

.rl-code-block__title { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.rl-code-block__language {
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
  /* Lowercase by convention, but never uppercased: `TypeScript` is a name. */
  font-family: var(--rl-font-family-mono);
}

.rl-code-block__actions {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  margin-left: auto;
}

.rl-code-block__copied {
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-success-fg);
}

.rl-code-block__pre {
  margin: 0;
  padding: var(--rl-space-3);
  overflow: auto;
  font-family: var(--rl-font-family-mono);
  font-size: var(--rl-font-size-sm);
  line-height: var(--rl-line-height-normal);
  color: var(--rl-color-text);
  /*
   * Preserved whitespace with wrapping off: code that wraps silently changes
   * what it means to read, and a horizontal scrollbar is the honest answer.
   */
  white-space: pre;
  tab-size: 2;
}

.rl-code-block__pre:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 var(--rl-focus-ring-width) var(--rl-color-focus);
}

/*
 * Line numbers as a grid per line, so the numbers form a column that a long
 * line cannot push out of alignment. Each line is its own row because the
 * markup carries no newlines in this mode: laying them out is what stacks them.
 */
.rl-code-block__pre--numbered .rl-code-block__line {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: var(--rl-space-3);
}

/*
 * The number column is as wide as its widest entry, which `ch` units get right
 * for a mono face: three digits for a file of a few hundred lines.
 */
.rl-code-block__pre--numbered .rl-code-block__number { min-width: 2ch; }

/* Keeps the line's own whitespace, which the grid cell would otherwise trim. */
.rl-code-block__text { white-space: pre; }

.rl-code-block__number {
  text-align: right;
  color: var(--rl-color-text-subtle);
  /* Not selectable, so dragging over the block copies the code alone. */
  user-select: none;
  -webkit-user-select: none;
}
</style>
