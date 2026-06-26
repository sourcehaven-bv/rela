// <rela-editor> — a self-contained markdown editor Custom Element for custom
// apps (TKT-5F9V56). Bundled into a standalone IIFE (vite.editor.config.ts) and
// served at the reserved per-app path /api/v1/_apps/<id>/_rela-editor.js.
//
// PUBLIC CONTRACT (the swap seam — this and nothing else is supported):
//   - property  `value`       get/set markdown text, whitespace-exact
//   - attribute `placeholder` plain-text placeholder
//   - attribute `readonly`    boolean
//   - event     `input`       dispatched on every change (per keystroke)
//   - event     `change`      dispatched on blur/commit
//   - method    `focus()`
//
// Everything else (that it's EasyMDE/CodeMirror underneath, the toolbar, the
// generated DOM) is an UNSUPPORTED implementation detail. The element renders
// into its own LIGHT DOM (not shadow DOM) because EasyMDE is built on
// CodeMirror 5, which misbehaves inside a shadow root. The narrow API above is
// what lets the editor be swapped later without touching plugins.

import EasyMDE from 'easymde'
// CSS is imported with ?inline so Vite returns it as a STRING (not an emitted
// sibling file), letting the build stay a single self-contained IIFE. The
// strings are injected into <head> once, on first element mount.
import easymdeCSS from 'easymde/dist/easymde.min.css?inline'
import fontAwesomeCSS from 'font-awesome/css/font-awesome.min.css?inline'
// Font-face override: points FA's glyph webfont at the app-relative reserved
// path (_rela-editor.woff2), permitted by the app CSP's `font-src <base>`.
// Injected AFTER fontAwesomeCSS so it wins.
import fontOverrideCSS from './relaEditorFont.css?inline'

const TAG = 'rela-editor'
const STYLE_ID = 'rela-editor-styles'

// Inject the editor's stylesheets into the document head exactly once,
// regardless of how many <rela-editor> elements mount. Order matters: FA base,
// then the font-face override, then EasyMDE's own CSS.
function ensureStylesInjected(): void {
  if (document.getElementById(STYLE_ID)) return
  const style = document.createElement('style')
  style.id = STYLE_ID
  style.textContent = [fontAwesomeCSS, fontOverrideCSS, easymdeCSS].join('\n')
  document.head.appendChild(style)
}

class RelaEditorElement extends HTMLElement {
  private _editor: EasyMDE | null = null
  private _textarea: HTMLTextAreaElement | null = null
  // Holds the value set via the property before the element is connected (and
  // before EasyMDE exists). Flushed into the editor on connect.
  private _pendingValue = ''
  private _onCmChange: (() => void) | null = null
  private _onCmBlur: (() => void) | null = null

  static get observedAttributes(): string[] {
    return ['placeholder', 'readonly']
  }

  connectedCallback(): void {
    if (this._editor) return // already mounted (re-connect)
    this._mount()
  }

  disconnectedCallback(): void {
    this._unmount()
  }

  attributeChangedCallback(name: string): void {
    if (!this._editor) return
    if (name === 'readonly') {
      this._editor.codemirror.setOption('readOnly', this.hasAttribute('readonly'))
    }
    // EasyMDE has no live placeholder setter; placeholder is read at mount.
    // Changing it after mount is not part of the supported contract.
  }

  // --- public property: value (markdown text, whitespace-exact) ---
  get value(): string {
    return this._editor ? this._editor.value() : this._pendingValue
  }

  set value(v: string) {
    const next = v == null ? '' : String(v)
    if (this._editor) {
      // Only replace if different, so setting the same value doesn't reset
      // the cursor/scroll.
      if (this._editor.value() !== next) this._editor.value(next)
    } else {
      this._pendingValue = next
    }
  }

  // --- public method: focus() ---
  override focus(): void {
    if (this._editor) this._editor.codemirror.focus()
    else super.focus()
  }

  private _mount(): void {
    ensureStylesInjected()
    const ta = document.createElement('textarea')
    // Seed EasyMDE with the pending value via the textarea so the initial
    // content is exact (EasyMDE reads the textarea on construction).
    ta.value = this._pendingValue
    this.appendChild(ta)
    this._textarea = ta

    const editor = new EasyMDE({
      element: ta,
      initialValue: this._pendingValue,
      placeholder: this.getAttribute('placeholder') || '',
      spellChecker: false,
      autofocus: false,
      status: false,
      // Suppress EasyMDE's runtime <link> to the FA CDN — the glyphs are
      // bundled (and the font is served same-origin under the app base).
      autoDownloadFontAwesome: false,
      toolbar: [
        'bold',
        'italic',
        'heading',
        '|',
        'unordered-list',
        'ordered-list',
        '|',
        'link',
        'code',
        'quote',
        '|',
        'preview',
        'side-by-side',
        'fullscreen',
        '|',
        'guide',
      ],
      minHeight: '200px',
    } satisfies EasyMDE.Options)

    if (this.hasAttribute('readonly')) {
      editor.codemirror.setOption('readOnly', true)
    }

    // Re-dispatch CodeMirror events as native element events so plugins use
    // standard addEventListener('input'/'change').
    this._onCmChange = () => {
      this.dispatchEvent(new Event('input', { bubbles: true }))
    }
    this._onCmBlur = () => {
      this.dispatchEvent(new Event('change', { bubbles: true }))
    }
    editor.codemirror.on('change', this._onCmChange)
    editor.codemirror.on('blur', this._onCmBlur)

    this._editor = editor
  }

  private _unmount(): void {
    if (this._editor) {
      if (this._onCmChange) this._editor.codemirror.off('change', this._onCmChange)
      if (this._onCmBlur) this._editor.codemirror.off('blur', this._onCmBlur)
      // Capture the current value back to _pendingValue so a disconnect →
      // reconnect round-trip preserves content.
      this._pendingValue = this._editor.value()
      // toTextArea() restores the original <textarea> and tears down CM.
      this._editor.toTextArea()
      this._editor.cleanup()
      this._editor = null
    }
    this._onCmChange = null
    this._onCmBlur = null
    if (this._textarea && this._textarea.parentNode === this) {
      this.removeChild(this._textarea)
    }
    this._textarea = null
  }
}

if (!customElements.get(TAG)) {
  customElements.define(TAG, RelaEditorElement)
}
