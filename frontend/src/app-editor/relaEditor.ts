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
// Theme override: re-skins EasyMDE's hardcoded light chrome with rela tokens so
// the editor follows the host light/dark theme. Injected LAST so it wins over
// EasyMDE's own CSS.
import themeCSS from './relaEditorTheme.css?inline'
import { attachBacktickAutocomplete, type BacktickController } from './relaBacktick'

// The bridge SDK (_rela.js) exposes window.rela. The backtick autocomplete only
// needs three read methods; declare the minimal shape we consume.
interface RelaBridge {
  schema(): Promise<{ entities: Record<string, { label?: string; id_prefix?: string; id_type?: string }> }>
  search(p: { query: string; type?: string }): Promise<{ data?: Array<{ id: string }> }>
  list(p: { type: string; params?: Record<string, unknown> }): Promise<{ data?: Array<{ id: string }> }>
}

const TAG = 'rela-editor'
const STYLE_ID = 'rela-editor-styles'

// Inject the editor's stylesheets into the document head exactly once,
// regardless of how many <rela-editor> elements mount. Order matters: FA base,
// then the font-face override, then EasyMDE's own CSS.
function ensureStylesInjected(): void {
  if (document.getElementById(STYLE_ID)) return
  const style = document.createElement('style')
  style.id = STYLE_ID
  style.textContent = [fontAwesomeCSS, fontOverrideCSS, easymdeCSS, themeCSS].join('\n')
  document.head.appendChild(style)
}

class RelaEditorElement extends HTMLElement {
  private _editor: EasyMDE | null = null
  private _textarea: HTMLTextAreaElement | null = null
  // Set only when EasyMDE construction failed and we fell back to the raw
  // <textarea>. When non-null, the value/focus contract routes through it.
  private _fallbackTextarea: HTMLTextAreaElement | null = null
  // Backtick entity-reference autocomplete, attached when window.rela is present.
  private _backtick: BacktickController | null = null
  // Holds the value set via the property before the element is connected (and
  // before EasyMDE exists). Flushed into the editor on connect.
  private _pendingValue = ''
  private _onCmChange: (() => void) | null = null
  private _onCmBlur: (() => void) | null = null
  private _onCmFocus: (() => void) | null = null
  // True while the .value setter is writing to EasyMDE, so the re-dispatched
  // input event is suppressed (programmatic sets are silent, like a textarea).
  private _settingValue = false
  // True between connectedCallback and the deferred mount running.
  private _mountScheduled = false
  // Value snapshot at focus, so `change` fires on blur ONLY when the content
  // actually changed — matching native <textarea>, not on every focus-out.
  private _valueAtFocus = ''

  // `placeholder` is intentionally NOT observed: EasyMDE reads it once at mount
  // and has no live setter, so observing it would invite a "set it reactively,
  // it silently no-ops after mount" footgun. It is a mount-time-only attribute.
  static get observedAttributes(): string[] {
    return ['readonly']
  }

  connectedCallback(): void {
    if (this._editor || this._mountScheduled) return // already mounted/scheduled
    // Defer the (heavy) EasyMDE/CodeMirror mount off the synchronous parse/
    // upgrade path so it doesn't block the main thread while the page is still
    // parsing (responsiveness). NOTE: this is a PERF measure, not a correctness
    // fix for the bridge handshake — readiness is made not-missable by the
    // SDK's replayable rela.ready/whenReady (see apps_sdk.go); do not rely on
    // this defer for that.
    this._mountScheduled = true
    queueMicrotask(() => {
      this._mountScheduled = false
      if (this.isConnected && !this._editor) this._mount()
    })
  }

  disconnectedCallback(): void {
    this._unmount()
  }

  attributeChangedCallback(name: string): void {
    if (!this._editor) return
    if (name === 'readonly') {
      this._editor.codemirror.setOption('readOnly', this.hasAttribute('readonly'))
    }
  }

  // --- public property: value (markdown text, whitespace-exact) ---
  get value(): string {
    if (this._editor) return this._editor.value()
    if (this._fallbackTextarea) return this._fallbackTextarea.value
    return this._pendingValue
  }

  set value(v: string) {
    const next = v == null ? '' : String(v)
    if (this._editor) {
      // Only replace if different, so setting the same value doesn't reset
      // the cursor/scroll.
      if (this._editor.value() !== next) {
        // Programmatic set MUST NOT emit input/change — same as a native
        // <textarea>/<input>, whose .value setter is silent. EasyMDE's
        // value() drives CodeMirror's change handler, so guard the dispatch.
        this._settingValue = true
        try {
          this._editor.value(next)
        } finally {
          this._settingValue = false
        }
      }
    } else if (this._fallbackTextarea) {
      // Native textarea .value is already silent — no dispatch guard needed.
      this._fallbackTextarea.value = next
    } else {
      this._pendingValue = next
    }
  }

  // --- public method: focus() ---
  override focus(): void {
    if (this._editor) this._editor.codemirror.focus()
    else if (this._fallbackTextarea) this._fallbackTextarea.focus()
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

    let editor: EasyMDE
    try {
      editor = new EasyMDE({
        element: ta,
        initialValue: this._pendingValue,
        placeholder: this.getAttribute('placeholder') || '',
        spellChecker: false,
        autofocus: false,
        status: false,
        // Suppress EasyMDE's runtime <link> to the FA CDN — the glyphs are
        // bundled (and the font is served same-origin under the app base).
        autoDownloadFontAwesome: false,
        // NOTE: this toolbar/options config is intentionally kept close to the
        // SPA editor's (frontend/src/components/forms/MarkdownEditor.vue) so the
        // two editors feel the same. They are NOT yet shared (this is a plain
        // IIFE with no Vue). If you change one, change the other — or do the
        // extraction tracked in TKT-D2JML7 (shared core). The app editor omits
        // the SPA's entity-ref toolbar button (it needs the schema store).
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
    } catch (err) {
      // Degraded fallback: if EasyMDE fails to construct (e.g. CM5 in an
      // unexpected DOM state), keep the raw <textarea> — it already holds the
      // seeded value — and route the value/focus/event contract through it so
      // the app still has a working (plain) editor rather than a dead element.
      console.error('[rela-editor] editor failed to initialize; falling back to a plain textarea', err)
      ta.placeholder = this.getAttribute('placeholder') || ''
      ta.readOnly = this.hasAttribute('readonly')
      ta.style.cssText = 'width:100%;min-height:200px;box-sizing:border-box'
      ta.addEventListener('input', () => {
        if (!this._settingValue) this.dispatchEvent(new Event('input', { bubbles: true }))
      })
      ta.addEventListener('change', () => this.dispatchEvent(new Event('change', { bubbles: true })))
      this._fallbackTextarea = ta
      // Keep _pendingValue in sync so the getter (which checks _fallbackTextarea)
      // and a later re-read are consistent.
      return
    }

    if (this.hasAttribute('readonly')) {
      editor.codemirror.setOption('readOnly', true)
    }

    // Re-dispatch CodeMirror events as native element events so plugins use
    // standard addEventListener('input'/'change'). Programmatic value sets
    // (the .value setter) are suppressed via _settingValue so they stay
    // silent like a native <textarea> — otherwise loading content into the
    // editor would spuriously fire input and trigger autosave loops.
    this._onCmChange = () => {
      if (this._settingValue) return
      this.dispatchEvent(new Event('input', { bubbles: true }))
    }
    // `change` matches native <textarea> semantics: fire on blur ONLY when the
    // value changed since focus, so consumers wiring autosave/dirty-tracking to
    // `change` don't get a spurious save on every click-away.
    this._onCmFocus = () => {
      this._valueAtFocus = editor.value()
    }
    this._onCmBlur = () => {
      if (editor.value() !== this._valueAtFocus) {
        this.dispatchEvent(new Event('change', { bubbles: true }))
      }
    }
    editor.codemirror.on('change', this._onCmChange)
    editor.codemirror.on('focus', this._onCmFocus)
    editor.codemirror.on('blur', this._onCmBlur)

    this._editor = editor
    // _pendingValue's job is done (EasyMDE now owns the content); clear it so a
    // large initial document isn't held twice. Invariant: _pendingValue is only
    // meaningful while _editor is null (the getter/setter both check _editor
    // first); _unmount repopulates it from the live value on teardown.
    this._pendingValue = ''

    // Attach backtick entity-reference autocomplete when the bridge is present.
    // The app sets window.rela via _rela.js; if it's absent (editor used without
    // the bridge) we just skip completion — the editor still works.
    const bridge = (window as unknown as { rela?: RelaBridge }).rela
    if (bridge && typeof bridge.schema === 'function') {
      try {
        this._backtick = attachBacktickAutocomplete(editor, bridge)
      } catch (err) {
        console.error('[rela-editor] backtick autocomplete failed to attach', err)
      }
    }
  }

  private _unmount(): void {
    if (this._backtick) {
      this._backtick.destroy()
      this._backtick = null
    }
    if (this._editor) {
      if (this._onCmChange) this._editor.codemirror.off('change', this._onCmChange)
      if (this._onCmFocus) this._editor.codemirror.off('focus', this._onCmFocus)
      if (this._onCmBlur) this._editor.codemirror.off('blur', this._onCmBlur)
      // Exit fullscreen BEFORE teardown. EasyMDE's fullscreen sets
      // document.body.style.overflow='hidden' and only restores it on toggle-
      // off; toTextArea()/cleanup() don't. Tearing down while fullscreen would
      // leave the host page permanently unscrollable with no editor to fix it.
      // `fullScreen` is a runtime CodeMirror option added by EasyMDE's
      // fullscreen addon (not in CM's typed EditorConfiguration); toggleFullScreen
      // is exposed statically.
      const cm = this._editor.codemirror as unknown as { getOption(name: string): unknown }
      if (cm.getOption('fullScreen')) {
        EasyMDE.toggleFullScreen(this._editor)
      }
      // Capture the current value back to _pendingValue so a disconnect →
      // reconnect round-trip preserves content.
      this._pendingValue = this._editor.value()
      // toTextArea() restores the original <textarea> and tears down CM.
      this._editor.toTextArea()
      this._editor.cleanup()
      this._editor = null
    }
    this._onCmChange = null
    this._onCmFocus = null
    this._onCmBlur = null
    // Preserve content across a fallback disconnect → reconnect too.
    if (this._fallbackTextarea) {
      this._pendingValue = this._fallbackTextarea.value
      this._fallbackTextarea = null
    }
    if (this._textarea && this._textarea.parentNode === this) {
      this.removeChild(this._textarea)
    }
    this._textarea = null
  }
}

if (!customElements.get(TAG)) {
  customElements.define(TAG, RelaEditorElement)
}
