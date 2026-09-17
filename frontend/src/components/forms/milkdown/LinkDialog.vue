<script setup lang="ts">
/**
 * The dialog for entering or editing a link target.
 *
 * Shaped like `EntityPickerModal`: teleported to `body`, focus moved in on
 * open and restored on close, registered with the shared modal stack. That
 * registration is not bookkeeping — `useKeyboardShortcuts` stands down while a
 * modal is open, and without it Escape would both close this dialog and, on a
 * form page, trigger `router.back()`, losing the user's edits.
 *
 * It validates before emitting. A refusal never reaches the document, and the
 * message it shows names the rule rather than echoing what was typed, so a
 * crafted URL cannot put content into the error surface.
 */
import { ref, watch, computed, nextTick, onBeforeUnmount } from 'vue'
import { useModalStack } from '@/composables/modalStack'
import { normalizeLinkUrl } from './linkUrl'

const props = defineProps<{
  open: boolean
  /** Existing target when editing; empty when inserting. */
  initialUrl?: string
  /**
   * Text the link will carry.
   *
   * When the caret is collapsed there is nothing to wrap, so the dialog asks
   * for the text too and inserts it. With a selection this is display-only:
   * the selected text is what gets marked.
   */
  initialText?: string
  /** True when the caret is collapsed, so the text field is needed. */
  needsText: boolean
  /** True when editing an existing link, which changes the labels. */
  editing: boolean
}>()

const emit = defineEmits<{
  /** A validated, normalized URL. Never the raw input. */
  submit: [payload: { url: string; text: string; strippedParams: boolean }]
  close: []
}>()

const urlInput = ref('')
const textInput = ref('')
const error = ref('')
const urlRef = ref<HTMLInputElement | null>(null)
const textRef = ref<HTMLInputElement | null>(null)
const dialogRef = ref<HTMLElement | null>(null)
const previouslyFocused = ref<HTMLElement | null>(null)

useModalStack(computed(() => props.open))

const title = computed(() => (props.editing ? 'Edit link' : 'Insert link'))
const submitLabel = computed(() => (props.editing ? 'Save' : 'Insert'))

// Watches the seed values ALONGSIDE `open`, not `open` alone.
//
// The obvious version — `watch(() => props.open, …)` reading `props.initialUrl`
// in the body — silently opens the dialog empty. All four props change in one
// update, and the watcher fires while that update is still in progress, so the
// body reads the PREVIOUS `initialUrl`. Watching the tuple makes the
// dependency real rather than incidental.
watch(
  () => [props.open, props.initialUrl, props.initialText] as const,
  ([isOpen, url, text], prev) => {
    const wasOpen = prev?.[0] ?? false
    if (isOpen && !wasOpen) {
      urlInput.value = url ?? ''
      textInput.value = text ?? ''
      error.value = ''
      previouslyFocused.value = (document.activeElement as HTMLElement) ?? null
      // Focus the URL field even when the text field is present: the URL is
      // what the user came here to supply, and the text is usually already
      // right.
      void nextTick(() => urlRef.value?.focus())
    } else if (!isOpen && wasOpen) {
      const prev2 = previouslyFocused.value
      previouslyFocused.value = null
      // Focus goes back where it came from, which is the editor at the caret.
      if (prev2?.isConnected) prev2.focus()
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  previouslyFocused.value = null
})

function onSubmit(): void {
  const result = normalizeLinkUrl(urlInput.value)
  if (!result.ok) {
    error.value = result.message
    void nextTick(() => urlRef.value?.focus())
    return
  }

  const text = props.needsText ? textInput.value.trim() : (props.initialText ?? '')
  if (props.needsText && !text) {
    error.value = 'Enter the text to show for this link.'
    void nextTick(() => textRef.value?.focus())
    return
  }

  emit('submit', {
    url: result.url,
    text,
    strippedParams: result.strippedParams === true,
  })
}

/**
 * Keeps Tab inside the dialog while it is open.
 *
 * A modal that lets focus wander to the page behind it is a modal in
 * appearance only; a keyboard user would be editing a form they cannot see.
 */
function trapTab(event: KeyboardEvent): void {
  const root = dialogRef.value
  if (!root) return
  const focusable = root.querySelectorAll<HTMLElement>(
    'input:not([disabled]), button:not([disabled])'
  )
  if (focusable.length === 0) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const active = document.activeElement

  if (event.shiftKey && active === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && active === last) {
    event.preventDefault()
    first.focus()
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    event.preventDefault()
    // Stop the global handler seeing this too. It bails while a modal is
    // registered, but a stopped event costs nothing and survives a future
    // change to that guard.
    event.stopPropagation()
    emit('close')
    return
  }
  if (event.key === 'Tab') trapTab(event)
}

/** Only a click on the backdrop itself closes, not one that bubbled from inside. */
function onOverlayClick(event: MouseEvent): void {
  if (event.target === event.currentTarget) emit('close')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="props.open"
      class="link-dialog-overlay"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      @click="onOverlayClick"
      @keydown="onKeydown"
    >
      <div ref="dialogRef" class="link-dialog">
        <h2 class="link-dialog-title">{{ title }}</h2>

        <form class="link-dialog-form" @submit.prevent="onSubmit">
          <label v-if="props.needsText" class="link-dialog-field">
            <span class="link-dialog-label">Text</span>
            <input
              ref="textRef"
              v-model="textInput"
              type="text"
              class="link-dialog-input"
              autocomplete="off"
              spellcheck="false"
            />
          </label>

          <label class="link-dialog-field">
            <span class="link-dialog-label">Address</span>
            <input
              ref="urlRef"
              v-model="urlInput"
              type="text"
              class="link-dialog-input"
              :class="{ 'has-error': error }"
              placeholder="https://example.com"
              autocomplete="off"
              spellcheck="false"
              :aria-invalid="Boolean(error)"
              aria-describedby="link-dialog-error"
              @input="error = ''"
            />
          </label>

          <!-- Always present so a screen reader announces the message when it
               arrives, rather than the region appearing along with it. -->
          <p id="link-dialog-error" class="link-dialog-error" role="alert" aria-live="polite">
            {{ error }}
          </p>

          <div class="link-dialog-actions">
            <button type="button" class="btn" @click="emit('close')">Cancel</button>
            <button type="submit" class="btn btn-primary">{{ submitLabel }}</button>
          </div>
        </form>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.link-dialog-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 12vh;
  z-index: 10000;
}

.link-dialog {
  background: var(--card-bg);
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
  width: 90%;
  max-width: 460px;
  padding: var(--space-lg);
}

.link-dialog-title {
  margin: 0 0 var(--space-md);
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--text-color);
}

.link-dialog-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.link-dialog-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.link-dialog-label {
  font-size: var(--font-size-sm);
  color: var(--text-muted);
}

.link-dialog-input {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--input-bg, var(--card-bg));
  color: var(--text-color);
  font-size: var(--font-size-base);
}

.link-dialog-input:focus-visible {
  outline: none;
  border-color: var(--accent-color);
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}

.link-dialog-input.has-error {
  border-color: var(--error-color, #dc2626);
}

.link-dialog-input.has-error:focus-visible {
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--error-ring);
}

/* Reserves its line whether or not there is a message, so showing one does
   not shove the buttons down under the pointer. */
.link-dialog-error {
  margin: 0;
  min-height: 1.2em;
  font-size: var(--font-size-sm);
  color: var(--error-color, #dc2626);
}

.link-dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
  margin-top: var(--space-sm);
}

.link-dialog-actions .btn:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}
</style>
