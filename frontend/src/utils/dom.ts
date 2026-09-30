/**
 * Check if an interactive element (input, textarea, select, contenteditable) is focused.
 * Used by keyboard shortcut handlers to avoid capturing keys during text input.
 */
export function isInputFocused(): boolean {
  const el = document.activeElement
  if (!el) return false
  const tag = el.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  if ((el as HTMLElement).isContentEditable) return true
  // Check for CodeMirror (EasyMDE)
  if (el.closest && el.closest('.CodeMirror')) return true
  return false
}

// Input types where Delete and Backspace edit nothing, so a shortcut may take
// them. A row checkbox is the common case: ticking one moves focus to it.
const NON_TEXT_INPUTS = new Set(['checkbox', 'radio', 'button', 'submit', 'reset'])

/**
 * Like isInputFocused, but a focused checkbox, radio or button does not count:
 * for keys such as Delete and Backspace, only a place that edits text does.
 */
export function isTextEntryFocused(): boolean {
  const el = document.activeElement
  if (el instanceof HTMLInputElement) return !NON_TEXT_INPUTS.has(el.type)
  return isInputFocused()
}
