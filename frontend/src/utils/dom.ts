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
