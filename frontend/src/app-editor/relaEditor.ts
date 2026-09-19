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
// Everything else (that it's Milkdown/ProseMirror underneath, the toolbar, the
// generated DOM) is an UNSUPPORTED implementation detail. The element renders
// into its own LIGHT DOM (not shadow DOM): ProseMirror works in a shadow root,
// but the app's own _rela.css theme tokens would not reach it, and the editor
// is meant to look like the app it sits in. The narrow API above is what lets
// the editor be swapped without touching plugins — it survived the EasyMDE →
// Milkdown move unchanged, which is what it was written for.
//
// This is the same editor the SPA runs (TKT-D2JML7). The command catalogue, the
// active/availability probes, the entityRef node, the serializer contract and
// the write-back guard are all imported from the SPA's modules rather than
// re-implemented, so the two editors cannot drift in what they produce. What is
// local is everything that needs a framework in the SPA: the toolbar and the
// `@` menu are built in plain DOM here.
//
// ONE deliberate difference: entity references render as BARE IDS, with no
// title. The SPA resolves titles from the server's per-principal `mentions`
// map; the app bridge has no such endpoint, and deriving a title any other way
// would route around the read gate (BUG-R9EHKV).

import {
  Editor,
  rootCtx,
  defaultValueCtx,
  editorViewOptionsCtx,
  editorViewCtx,
} from '@milkdown/kit/core'
import { gfm } from '@milkdown/kit/preset/gfm'
import { history } from '@milkdown/kit/plugin/history'
import { cursor } from '@milkdown/kit/plugin/cursor'
import { trailing } from '@milkdown/kit/plugin/trailing'
import { SlashProvider, slashFactory } from '@milkdown/kit/plugin/slash'
import { replaceAll, getMarkdown, callCommand, $prose } from '@milkdown/kit/utils'
import { Plugin, PluginKey, TextSelection } from '@milkdown/kit/prose/state'
import type { EditorState } from '@milkdown/kit/prose/state'
import type { EditorView } from '@milkdown/kit/prose/view'
import { lift } from '@milkdown/kit/prose/commands'

import { RELA_COMMONMARK, configureRelaSerializer } from '@/components/forms/milkdown/editorPreset'
import { relaCommentNode, relaCommentRemarkPlugin } from '@/components/forms/milkdown/commentNode'
import { entityRefNode, isValidEntityRefId } from '@/components/forms/milkdown/entityRefNode'
import { guardWriteBack } from '@/components/forms/milkdown/writeBackGuard'
import { parseMentionQuery } from '@/components/forms/milkdown/mentionQuery'
import {
  INLINE_COMMANDS,
  BLOCK_COMMANDS,
  type EditorCommand,
} from '@/components/forms/milkdown/editorCommands'
import { activeCommandIds } from '@/components/forms/milkdown/activeFormats'
import { unavailableCommandIds } from '@/components/forms/milkdown/commandAvailability'
import {
  TABLE_COMMANDS,
  DIRECT_TABLE_COMMANDS,
  GUARDED_TABLE_COMMANDS,
  canRunTableCommand,
  canAddRowBefore,
  runTableCommand,
  isInTable,
} from '@/components/forms/milkdown/tableCommands'
import { createToolbar, type ToolbarHandle } from './relaToolbar'
import {
  createMentionMenu,
  type MentionMenuHandle,
  type MentionSearchBridge,
} from './relaMentionMenu'

// The editor's CSS is NOT imported here. It is concatenated at build time by
// the emitEditorCSS plugin (vite.editor.config.ts) into a sibling
// rela-editor.css, which rela serves at the app-relative reserved path
// _rela-editor.css. This module only links it — see ensureStylesInjected for
// why an inline <style> is not an option.

/** The bridge SDK (_rela.js) exposes window.rela. Only `search` is used. */
type RelaBridge = MentionSearchBridge

const TAG = 'rela-editor'
const STYLE_ID = 'rela-editor-styles'
// The app CSP has no 'unsafe-inline', so an injected <style> element would be
// blocked and the editor would render unstyled. rela serves the same CSS as a
// file at the app's own path; a <link> is a resource load, which the
// path-scoped style-src permits. Relative on purpose: it resolves against the
// app base (/api/v1/_apps/<id>/), like _rela.js.
//
// Note this constrains ONLY injected stylesheets. ProseMirror positions its
// own chrome by writing DOM style PROPERTIES (`el.style.left`), which the same
// CSP allows — `style-src` governs stylesheets and the `style` ATTRIBUTE, not
// the CSSOM. Verified against a server replicating the real app CSP before the
// port: zero violations, block handle and tables included.
const STYLE_HREF = '_rela-editor.css'

const ALL_COMMANDS: EditorCommand[] = [...INLINE_COMMANDS, ...BLOCK_COMMANDS, ...TABLE_COMMANDS]

// Link the editor's stylesheet into the document head exactly once, regardless
// of how many <rela-editor> elements mount.
//
// A <link> rather than a <style> with inlined text: the app CSP carries no
// 'unsafe-inline', so an injected <style> element is blocked outright — the
// element lands in the DOM, its .sheet stays null, and the editor renders with
// no styling at all. That failure is silent apart from a console violation,
// which is why the served stylesheet is the only supported path.
function ensureStylesInjected(): void {
  if (document.getElementById(STYLE_ID)) return
  const link = document.createElement('link')
  link.id = STYLE_ID
  link.rel = 'stylesheet'
  link.href = STYLE_HREF
  // Say so if it does not arrive. A missing stylesheet leaves a fully
  // functional but completely unstyled editor, which looks like a rendering bug
  // rather than a missing file — and the most likely cause (a checkout where
  // the frontend build has not run, so the embedded asset is empty) gives no
  // other signal at all. The editor deliberately keeps working: an app that can
  // still capture text is better than one that cannot.
  link.addEventListener('error', () => {
    console.error(
      `[rela-editor] could not load ${STYLE_HREF}; the editor will work but ` +
        `render unstyled. Is the editor bundle built and served?`
    )
  })
  document.head.appendChild(link)
}

const dirtyTrackerKey = new PluginKey('rela-app-editor-dirty-tracker')

/**
 * Marks a transaction as the editor withdrawing a trigger it typed itself.
 *
 * The dirty tracker skips these, so pressing the entity-reference button and
 * then dismissing the menu leaves the document exactly as it was — text AND
 * edited-state — rather than disarming the write-back guard for the session.
 */
const WITHDRAW_PROMPT_META = 'rela-withdraw-ref-prompt'

class RelaEditorElement extends HTMLElement {
  private _editor: Editor | null = null
  private _shell: HTMLElement | null = null
  private _placeholderEl: HTMLElement | null = null
  private _toolbar: ToolbarHandle | null = null
  private _menu: MentionMenuHandle | null = null
  private _menuAnchor: HTMLElement | null = null
  private _slashProvider: SlashProvider | null = null
  // Set only when the editor failed to construct and we fell back to a raw
  // <textarea>. When non-null, the value/focus contract routes through it.
  private _fallbackTextarea: HTMLTextAreaElement | null = null
  // Holds the value set via the property before the element is connected (and
  // before the editor exists). Flushed into the editor on connect.
  private _pendingValue = ''
  // True while the .value setter is writing to the editor, so the input event
  // is suppressed (programmatic sets are silent, like a native textarea).
  private _settingValue = false
  // True between connectedCallback and the deferred mount running.
  private _mountScheduled = false
  // Value snapshot at focus, so `change` fires on blur ONLY when the content
  // actually changed — matching native <textarea>, not on every focus-out.
  private _valueAtFocus = ''
  // The EXACT bytes last handed to the editor, kept pristine for the write-back
  // guard. Only the .value setter and the initial mount assign it.
  private _originalValue = ''
  // Whether the user has changed the document since it was loaded.
  private _dirty = false
  // Whether document changes now count as user edits. False while a document is
  // loading, because loading provokes normalization that is not an edit.
  private _armed = false
  // Where the active `@` query starts, so insertion replaces trigger and query.
  private _activeMatchLength = 0
  // The `@` query the user dismissed with Escape. SlashProvider re-decides
  // visibility on every update, so closing the menu in the key handler alone
  // let it reappear on the very next keystroke.
  private _dismissedQuery: string | null = null
  // The span `_promptForRef` typed into the document, so dismissing the menu
  // can take it back out. Null whenever the trigger was the user's own typing.
  //
  // `dirtyBefore` records what the edited-state was before the button was
  // pressed. It is NOT restored on withdrawal — see `_withdrawPromptedRef` for
  // why that would be a lie — but it is what a future fix would need, so it is
  // captured rather than reconstructed.
  private _promptedRefSpan: {
    from: number
    to: number
    dirtyBefore: boolean
    /** The editor's serialization just before the trigger was typed. */
    serializedBefore: string
  } | null = null
  private _onKeydown: ((e: KeyboardEvent) => void) | null = null

  // `placeholder` is intentionally NOT observed: it is read once at mount, so
  // observing it would invite a "set it reactively, it silently no-ops after
  // mount" footgun. It is a mount-time-only attribute.
  static get observedAttributes(): string[] {
    return ['readonly']
  }

  connectedCallback(): void {
    if (this._editor || this._mountScheduled) return // already mounted/scheduled
    // Defer the (heavy) editor mount off the synchronous parse/upgrade path so
    // it doesn't block the main thread while the page is still parsing
    // (responsiveness). NOTE: this is a PERF measure, not a correctness fix for
    // the bridge handshake — readiness is made not-missable by the SDK's
    // replayable rela.ready/whenReady (see apps_sdk.go); do not rely on this
    // defer for that.
    this._mountScheduled = true
    queueMicrotask(() => {
      this._mountScheduled = false
      if (this.isConnected && !this._editor) void this._mount()
    })
  }

  disconnectedCallback(): void {
    this._unmount()
  }

  attributeChangedCallback(name: string): void {
    if (name !== 'readonly') return
    if (this._fallbackTextarea) {
      this._fallbackTextarea.readOnly = this.hasAttribute('readonly')
      return
    }
    this._applyReadonly()
  }

  // --- public property: value (markdown text, whitespace-exact) ---
  get value(): string {
    if (this._fallbackTextarea) return this._fallbackTextarea.value
    if (!this._editor) return this._pendingValue
    // The write-back guard, on the read rather than beside it. The getter IS
    // the app's save path, so this is the only place it can sit.
    //
    // WHAT IT GUARANTEES, and what it does not. A WYSIWYG editor round-trips
    // every body it opens. Until the user edits something, the guard returns the
    // ORIGINAL BYTES, so an app that merely displayed a body cannot write a diff
    // for it. Once the user edits, the guard steps aside and the app gets the
    // full reserialization — including reformatting of parts of the document
    // nobody touched (a table repadded to its column widths, a setext heading
    // rewritten to ATX).
    //
    // That second half is a real behaviour change from the EasyMDE editor this
    // replaced, which was a plain text buffer and returned exactly what was
    // typed. It is inherent to editing a parsed document rather than text, so it
    // is not a bug to fix here — but it is a thing an app author has to know, so
    // it is documented in the custom-apps guide rather than left to be
    // discovered in a diff. `relaEditor.test.ts` pins both halves.
    //
    // A round-trip that changes MEANING is different again: that is a bug in the
    // parse/serialize pair, and the guard refuses it, costing the user an edit
    // rather than corrupting the body.
    return guardWriteBack(this._originalValue, this._serialize(), this._dirty).value
  }

  set value(v: string) {
    const next = v == null ? '' : String(v)
    if (this._fallbackTextarea) {
      // Native textarea .value is already silent — no dispatch guard needed.
      this._fallbackTextarea.value = next
      this._originalValue = next
      return
    }
    if (!this._editor) {
      this._pendingValue = next
      return
    }
    // Only replace if different, so setting the same value doesn't reset the
    // cursor/scroll.
    if (this._serialize() === next) return
    // Programmatic set MUST NOT emit input/change — same as a native
    // <textarea>/<input>, whose .value setter is silent.
    this._settingValue = true
    // A new body from the app: this is now the pristine baseline, and nothing
    // in it is the user's edit.
    this._originalValue = next
    this._dirty = false
    this._armed = false
    // Positions in the OLD document; see the note in `_unmount`.
    this._promptedRefSpan = null
    try {
      this._editor.action(replaceAll(next))
    } finally {
      this._settingValue = false
    }
    const view = this._view()
    if (view) this._refreshDerivedState(view.state)
    this._armed = true
  }

  // --- public method: focus() ---
  override focus(): void {
    if (this._fallbackTextarea) this._fallbackTextarea.focus()
    else if (this._editor) this._view()?.focus()
    else super.focus()
  }

  private _view(): EditorView | null {
    if (!this._editor) return null
    try {
      return this._editor.ctx.get(editorViewCtx)
    } catch {
      return null
    }
  }

  /** The markdown the editor currently holds, unguarded. */
  private _serialize(): string {
    if (!this._editor) return this._originalValue
    try {
      return this._editor.action(getMarkdown())
    } catch {
      return this._originalValue
    }
  }

  private _applyReadonly(): void {
    const readonly = this.hasAttribute('readonly')
    const editor = this._editor
    if (!editor) return
    try {
      editor.ctx.update(editorViewOptionsCtx, (prev) => ({
        ...prev,
        editable: () => !readonly,
      }))
    } catch {
      return
    }
    // The view caches `editable`; updating the ctx alone does not re-read it.
    const view = this._view()
    if (view) view.setProps({ editable: () => !readonly })
    this._shell?.classList.toggle('is-readonly', readonly)
  }

  private async _mount(): Promise<void> {
    ensureStylesInjected()

    const shell = document.createElement('div')
    shell.className = 'rela-editor-shell'
    this.appendChild(shell)
    this._shell = shell

    const bridge = (window as unknown as { rela?: RelaBridge }).rela
    const hasBridge = Boolean(bridge && typeof bridge.search === 'function')

    const toolbar = createToolbar(
      {
        run: (cmd) => this._runCommand(cmd),
        insertRef: () => this._promptForRef(),
      },
      hasBridge
    )
    shell.appendChild(toolbar.root)
    this._toolbar = toolbar

    const body = document.createElement('div')
    body.className = 'rela-editor-body'
    const editorRoot = document.createElement('div')
    editorRoot.className = 'rela-editor-surface'
    body.appendChild(editorRoot)
    shell.appendChild(body)

    // A real element rather than a `::before` fed by `attr()`: the placeholder
    // text would have to reach a pseudo-element through an inline `style`, and
    // the app CSP (`style-src` with no `'unsafe-inline'`) blocks the `style`
    // attribute outright.
    //
    // `aria-hidden` because ProseMirror's own textbox role already names the
    // field; a screen reader announcing both would read it twice.
    const placeholder = document.createElement('div')
    placeholder.className = 'rela-editor-placeholder'
    placeholder.setAttribute('aria-hidden', 'true')
    // `??`, not `||`: an app that sets `placeholder=""` is asking for NO
    // placeholder, exactly as on a native <textarea>, and must not be given the
    // default instead. The default applies only when the attribute is absent.
    placeholder.textContent = this.getAttribute('placeholder') ?? 'Markdown content...'
    editorRoot.appendChild(placeholder)
    this._placeholderEl = placeholder

    // Outside the editable surface on purpose. SlashProvider positions THIS
    // element and appends it to the editor's parent; leaving it inside the
    // contenteditable would make the menu part of the document being edited.
    const menuAnchor = document.createElement('div')
    menuAnchor.className = 'rela-mention-anchor'
    menuAnchor.dataset.show = 'false'
    shell.appendChild(menuAnchor)
    this._menuAnchor = menuAnchor

    if (bridge && hasBridge) {
      const menu = createMentionMenu(bridge, { pick: (id) => this._insertRef(id) })
      menuAnchor.appendChild(menu.root)
      this._menu = menu
    }

    this._originalValue = this._pendingValue

    try {
      this._editor = await this._createEditor(editorRoot)
    } catch (err) {
      // Degraded fallback: if the editor fails to construct, give the app a
      // working (plain) textarea rather than a dead element.
      console.error(
        '[rela-editor] editor failed to initialize; falling back to a plain textarea',
        err
      )
      this._mountFallback(shell)
      return
    }

    // The element may have been removed while the async create was in flight.
    // Without this the editor would be left mounted into a detached shell with
    // nothing to tear it down.
    if (!this.isConnected) {
      this._unmount()
      return
    }

    this._applyReadonly()

    const view = this._view()
    if (view) {
      // Seed the derived state from the loaded document. `appendTransaction`
      // only runs once a transaction is dispatched, so without this the toolbar
      // shows nothing active until the first edit.
      this._refreshDerivedState(view.state)
      view.dom.addEventListener('focus', this._onEditorFocus)
      view.dom.addEventListener('blur', this._onEditorBlur)
    }

    // Bound on the shell in the capture phase so it runs before ProseMirror's
    // own keymap: without that, Enter inserts a paragraph break and the arrow
    // keys move the cursor instead of the menu highlight.
    this._onKeydown = (e) => this._onKeydownCapture(e)
    shell.addEventListener('keydown', this._onKeydown, true)

    // _pendingValue's job is done (the editor now owns the content); clear it so
    // a large initial document isn't held twice. Invariant: _pendingValue is
    // only meaningful while _editor is null; _unmount repopulates it from the
    // live value on teardown.
    this._pendingValue = ''
    // Loading is done; from here a document change is the user's doing.
    this._armed = true
  }

  private _createEditor(root: HTMLElement): Promise<Editor> {
    const slash = slashFactory('rela-app-entity-mention')

    // Marks the editor dirty on a document change the USER caused.
    //
    // `docChanged` alone is not that signal: loading a document runs
    // normalization of its own (ProseMirror's table plugin rewrites column
    // widths before the user has touched anything). Counting that as an edit
    // would tell the write-back guard the body was edited, so it would write the
    // reformatted version back for an entity that was only opened. `_armed` is
    // what separates the two.
    const dirtyTracker = $prose(
      () =>
        new Plugin({
          key: dirtyTrackerKey,
          appendTransaction: (transactions, _oldState, newState) => {
            for (const tr of transactions) {
              // A withdrawn reference prompt is the editor undoing its own
              // insertion, not the user editing; see `_withdrawPromptedRef`.
              if (tr.getMeta(WITHDRAW_PROMPT_META)) continue
              if (this._armed && tr.docChanged) this._dirty = true
            }
            // Recomputed here rather than polled so the toolbar tracks the
            // selection as it moves, which changes state without changing the
            // document.
            this._refreshDerivedState(newState)
            return null
          },
          // `input` fires from the VIEW hook, not from `appendTransaction`.
          //
          // Two reasons, and the second is the one that bit. The event must fire
          // per keystroke, so it cannot come off the markdown listener, which is
          // debounced by 200ms — a change made and read inside that window is
          // exactly what an app saving on submit does. But `appendTransaction`
          // runs while the new state is still being assembled, so a listener
          // reading `.value` from it got the document as it was BEFORE the edit:
          // every event reported the previous value, and the last edit never
          // appeared at all. `update` runs after the view has the new state.
          //
          // The event says "something changed", not "one thing changed". A
          // compound command can land as more than one document transaction
          // (wrapping a paragraph in a heading is two, since the trailing plugin
          // appends a paragraph after it), so an app gets two events carrying the
          // same, correct, final value. Native `input` behaves the same way for a
          // paste that triggers reformatting.
          view: () => ({
            update: (view, prevState) => {
              if (this._settingValue) return
              if (view.state.doc.eq(prevState.doc)) return
              this.dispatchEvent(new Event('input', { bubbles: true }))
            },
          }),
        })
    )

    return (
      Editor.make()
        .config((ctx) => {
          ctx.set(rootCtx, root)
          ctx.set(defaultValueCtx, this._pendingValue)
          // `md-body` is the marker class for the shared markdown stylesheet
          // (styles/markdown-content.css), which the bundle concatenates. Wearing
          // it means the editing surface inherits the exact headings, lists,
          // tables, quotes and code styling the entity view will show, so WYSIWYG
          // is true by construction instead of by a parallel set of rules that
          // would drift. The SPA editor does the same.
          ctx.update(editorViewOptionsCtx, (prev) => ({
            ...prev,
            attributes: { ...(prev.attributes ?? {}), class: 'rela-editor-prose md-body' },
          }))
          // Shared with the SPA editor, so the two cannot serialize a body
          // differently.
          configureRelaSerializer(ctx)
          ctx.set(slash.key, {
            view: () => {
              this._slashProvider = new SlashProvider({
                content: this._menuAnchor as HTMLElement,
                trigger: '@',
                debounce: 0,
                // The default shouldShow only fires while `@` is the last
                // character typed, so it cannot follow a query. This reads the
                // text before the cursor on every update instead.
                shouldShow: (view) => {
                  if (!this._menu) return false
                  const match = parseMentionQuery(this._slashProvider?.getContent(view))
                  if (!match) {
                    if (this._menu.isOpen) this._menu.close()
                    this._dismissedQuery = null
                    // Cleared with the menu: it is the span `_insertRef` deletes,
                    // so a stale value would let a later insertion eat characters
                    // that are no longer part of a query.
                    this._activeMatchLength = 0
                    return false
                  }
                  // Escape dismissed exactly this query. Editing it (typing or
                  // deleting) produces a different one and the menu returns.
                  if (this._dismissedQuery !== null && match.query === this._dismissedQuery)
                    return false
                  this._dismissedQuery = null
                  this._activeMatchLength = match.matchLength
                  this._menu.setQuery(match.query)
                  return true
                },
              })
              return {
                update: (view, prevState) => this._slashProvider?.update(view, prevState),
                destroy: () => this._slashProvider?.destroy(),
              }
            },
          })
        })
        .use(RELA_COMMONMARK)
        .use(gfm)
        .use(history)
        .use(entityRefNode)
        // The remark half must load with the node: it retypes comment-only `html`
        // mdast nodes so the schema claims them instead of the preset's.
        .use(relaCommentRemarkPlugin)
        .use(relaCommentNode)
        .use(dirtyTracker)
        .use(slash)
        // `cursor` gives the gap cursor and `trailing` a way to click below a
        // table or code block that ends the document to start a new paragraph.
        // The SPA's drag handle (`block`) is left out: it needs floating-ui and a
        // gutter this narrower surface does not reserve.
        .use(cursor)
        .use(trailing)
        .create()
    )
  }

  private _mountFallback(shell: HTMLElement): void {
    this._toolbar?.destroy()
    this._toolbar = null
    this._menu?.destroy()
    this._menu = null
    this._placeholderEl = null
    shell.replaceChildren()

    const ta = document.createElement('textarea')
    ta.className = 'rela-editor-fallback'
    ta.value = this._pendingValue
    ta.placeholder = this.getAttribute('placeholder') ?? ''
    ta.readOnly = this.hasAttribute('readonly')
    ta.addEventListener('input', () => {
      if (!this._settingValue) this.dispatchEvent(new Event('input', { bubbles: true }))
    })
    ta.addEventListener('change', () => this.dispatchEvent(new Event('change', { bubbles: true })))
    shell.appendChild(ta)
    this._fallbackTextarea = ta
  }

  // `change` matches native <textarea> semantics: fire on blur ONLY when the
  // value changed since focus, so consumers wiring autosave/dirty-tracking to
  // `change` don't get a spurious save on every click-away.
  //
  // `_focused` is what makes that survive the toolbar. Every command ends by
  // returning focus to the document, which fires `focus` again — so snapshotting
  // unconditionally re-baselined to the POST-edit value, and clicking away after
  // formatting something from the toolbar emitted no `change` at all. Only a
  // focus arriving from outside the editor starts a new comparison window.
  private _focused = false

  private _onEditorFocus = (): void => {
    if (this._focused) return
    this._focused = true
    this._valueAtFocus = this._serialize()
  }

  private _onEditorBlur = (): void => {
    this._focused = false
    if (this._serialize() !== this._valueAtFocus) {
      this.dispatchEvent(new Event('change', { bubbles: true }))
    }
  }

  /** True when the document holds nothing a user has written. */
  private _isDocEmpty(state: EditorState): boolean {
    const { doc } = state
    if (doc.childCount === 0) return true
    if (doc.childCount > 1) return false
    const first = doc.firstChild
    return first !== null && first.type.name === 'paragraph' && first.content.size === 0
  }

  /** Recomputes everything the UI derives from editor state. */
  private _refreshDerivedState(state: EditorState): void {
    if (this._placeholderEl) this._placeholderEl.hidden = !this._isDocEmpty(state)
    const toolbar = this._toolbar
    const editor = this._editor
    if (!toolbar || !editor) return

    const active = activeCommandIds(state, ALL_COMMANDS)
    const unavailable = unavailableCommandIds(
      editor.ctx,
      state,
      ALL_COMMANDS.filter(
        (c) => !DIRECT_TABLE_COMMANDS.has(c.id) && !GUARDED_TABLE_COMMANDS.has(c.id)
      ),
      active,
      lift(state)
    )
    // The deletes are not registered slices, so they answer for themselves.
    for (const cmd of ALL_COMMANDS) {
      if (DIRECT_TABLE_COMMANDS.has(cmd.id) && !canRunTableCommand(cmd.id, state)) {
        unavailable.add(cmd.id)
      }
    }
    // AddRowBefore IS a slice, and it claims to apply in the header row before
    // corrupting the table. Its own guard overrides the dry run.
    if (!canAddRowBefore(state)) unavailable.add('addRowBefore')

    toolbar.update(active, unavailable, isInTable(state))
  }

  /** Runs a formatting command and returns focus to the document. */
  private _runCommand(cmd: EditorCommand): void {
    const editor = this._editor
    if (!editor) return
    const view = this._view()
    if (!view) return
    if (!view.editable) return

    // Row/column deletes bypass the command manager; see `tableCommands`.
    if (DIRECT_TABLE_COMMANDS.has(cmd.id)) {
      runTableCommand(cmd.id, view.state, view.dispatch.bind(view))
      view.focus()
      return
    }

    // An active block command runs its inverse, so the button toggles rather
    // than no-opping on a block that is already that type.
    const active = activeCommandIds(view.state, [cmd]).has(cmd.id)
    if (active && cmd.toggleTo === 'lift') {
      // Unwrapping a blockquote has no preset command; see `toggleTo`.
      lift(view.state, view.dispatch)
    } else if (active && cmd.toggleTo) {
      editor.action(callCommand(cmd.toggleTo))
    } else {
      editor.action(callCommand(cmd.command, cmd.payload))
    }
    this._view()?.focus()
  }

  /**
   * Opens the `@` menu from the toolbar button.
   *
   * The SPA has a modal entity picker for this. Building a second one here
   * would mean a second search UI to keep in step, so the button types the
   * trigger instead: the same menu, reached without knowing the shortcut.
   *
   * Typing it IS a document change, which matters more than it looks. Pressing
   * the button and then changing your mind would otherwise leave a stray `@`
   * behind AND mark the document dirty forever, so the next `.value` read hands
   * the app a fully reserialized body it never asked for. `_promptedRefSpan`
   * records what was inserted so dismissing the menu takes it back out.
   */
  private _promptForRef(): void {
    const view = this._view()
    if (!view || !view.editable) return
    view.focus()
    const { state } = view
    // Insert AT the selection end, never OVER the selection. `insertText(t, from,
    // to)` replaces the range, so passing the live selection deleted whatever the
    // user had highlighted: selecting `world` in `hello world` and pressing the
    // reference button left `hello  @`, with the word recoverable only by an undo
    // the user has no reason to expect after pressing an *insert* button.
    const { $to, to } = state.selection
    // A boundary before the `@` is what `parseMentionQuery` requires, so add one
    // when the cursor sits mid-word. Read at `to` rather than `from`: with a
    // selection those differ, and `from` looks at a character inside the range
    // the trigger is being appended after.
    const charBefore = to > $to.start() ? state.doc.textBetween(to - 1, to) : ' '
    const prefix = /[\s([\]{}>,;:"']/.test(charBefore) ? '@' : ' @'
    // BOTH captured before the dispatch. `view.dispatch` runs the dirty tracker
    // synchronously, so reading `this._dirty` after it always reports true and
    // the restore below would hand back the state the insertion itself caused.
    const serializedBefore = this._serialize()
    const dirtyBefore = this._dirty
    view.dispatch(state.tr.insertText(prefix, to, to))
    this._promptedRefSpan = {
      from: to,
      to: to + prefix.length,
      dirtyBefore,
      serializedBefore,
    }
  }

  /**
   * Removes a trigger this editor typed, when the user dismisses the menu.
   *
   * Only ever removes the exact span `_promptForRef` inserted, and only while it
   * still holds exactly that text — once the user has typed a query after it or
   * edited around it, the span is theirs and is left alone.
   *
   * The span is CONSUMED only on a path that resolves it: either the text was
   * removed, or it no longer matches so it is the user's now. Clearing it up
   * front instead meant a withdrawal that merely could not run yet — most
   * reachably because the app had set `readonly` in the meantime — threw the
   * span away, stranding the `@` for the rest of the session with no way back.
   */
  private _withdrawPromptedRef(): void {
    const span = this._promptedRefSpan
    if (!span) return
    const view = this._view()
    // Not consumed: the editor cannot act right now, but the span is still ours
    // and the next dismissal should be able to use it.
    if (!view || !view.editable) return
    const { state } = view
    if (span.to > state.doc.content.size) {
      // The document shrank past the span, so it cannot be the text we typed.
      this._promptedRefSpan = null
      return
    }
    const text = state.doc.textBetween(span.from, span.to)
    if (!/^ ?@$/.test(text)) {
      // The user built on it. Theirs now.
      this._promptedRefSpan = null
      return
    }
    // Tagged so the dirty tracker skips the removal itself — taking back the
    // editor's own insertion is not the user editing.
    const tr = state.tr.delete(span.from, span.to)
    tr.setMeta(WITHDRAW_PROMPT_META, true)
    view.dispatch(tr)
    this._promptedRefSpan = null

    // Restore the edited-state, so pressing the button and changing your mind
    // costs nothing.
    //
    // Gated on the OBSERVABLE, never on an argument about which plugins ran.
    // Withdrawal usually returns the document exactly, but not always: inserting
    // at the end of certain bodies makes the trailing plugin append a paragraph
    // that the delete does not remove. That paragraph is EMPTY, so it is
    // invisible in rendered text and only a serialization can see it — which is
    // how two separate attempts to reason about this reached opposite wrong
    // conclusions. Comparing what the editor now produces against what it
    // produced a moment before the trigger was typed decides it correctly
    // without anyone having to know why.
    //
    // The comparison is against that pre-prompt serialization, NOT against
    // `_originalValue`. A WYSIWYG round-trip reformats every body it opens, so
    // an unedited document already serializes differently from the bytes the app
    // supplied — comparing against those could never match, and the restore
    // would be dead code that looked like a fix.
    if (this._serialize() === span.serializedBefore) this._dirty = span.dirtyBefore
  }

  /** Replaces the `@query` before the cursor with an entityRef node. */
  private _insertRef(id: string): void {
    const view = this._view()
    if (!view) return
    if (!isValidEntityRefId(id)) return
    // No live query means no `@...` span to replace. Proceeding would delete
    // whatever happened to sit before the cursor.
    if (this._activeMatchLength <= 0) return

    const { state } = view
    const { $from } = state.selection
    const to = $from.pos
    // Replaces the trigger and the query together, so the `@abc` the user typed
    // does not survive alongside the node it produced.
    const from = Math.max($from.start(), to - this._activeMatchLength)

    // No title attribute: bare ID by design, see the module header.
    const node = state.schema.nodes.entityRef.create({ id })
    const tr = state.tr.replaceWith(from, to, node)
    // A space after the reference so the user can keep typing prose without the
    // next character being absorbed into the node.
    tr.insertText(' ', from + 1)
    tr.setSelection(TextSelection.create(tr.doc, from + 2))
    view.dispatch(tr)
    // The insertion replaced the trigger, so there is nothing left to withdraw.
    this._promptedRefSpan = null
    this._closeMenu()
    view.focus()
  }

  private _closeMenu(): void {
    this._menu?.close()
    this._slashProvider?.hide()
    this._activeMatchLength = 0
  }

  /** Keyboard handling while the `@` menu is open. */
  private _onKeydownCapture(event: KeyboardEvent): void {
    const menu = this._menu
    // Escape is handled BEFORE the open check. A trigger the toolbar typed has
    // to be withdrawable even when the menu never appeared — which is exactly
    // the case where a stray `@` would otherwise be stranded in the document
    // with nothing on screen to explain it.
    if (event.key === 'Escape' && this._promptedRefSpan) {
      event.preventDefault()
      this._closeMenu()
      this._withdrawPromptedRef()
      return
    }
    if (!menu || !menu.isOpen) return
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        menu.moveHighlight(1)
        break
      case 'ArrowUp':
        event.preventDefault()
        menu.moveHighlight(-1)
        break
      case 'Enter':
      case 'Tab': {
        const id = menu.current()
        if (!id) return
        event.preventDefault()
        this._insertRef(id)
        break
      }
      case 'Escape': {
        event.preventDefault()
        // Remembering WHICH query was dismissed keeps the menu shut until the
        // user types something else, rather than latching the trigger off.
        const view = this._view()
        const match = view ? parseMentionQuery(this._slashProvider?.getContent(view)) : null
        this._dismissedQuery = match?.query ?? null
        this._closeMenu()
        // Takes back a trigger the TOOLBAR typed, so pressing the reference
        // button and changing your mind leaves the document as it was.
        this._withdrawPromptedRef()
        break
      }
      default:
        break
    }
  }

  private _unmount(): void {
    // Capture the current value back to _pendingValue so a disconnect →
    // reconnect round-trip preserves content. Guarded, not raw: an app that
    // never edited must not get churn back on reconnect.
    if (this._editor) {
      this._pendingValue = guardWriteBack(this._originalValue, this._serialize(), this._dirty).value
    } else if (this._fallbackTextarea) {
      this._pendingValue = this._fallbackTextarea.value
    }

    if (this._onKeydown && this._shell) {
      this._shell.removeEventListener('keydown', this._onKeydown, true)
    }
    this._onKeydown = null

    const view = this._view()
    if (view) {
      view.dom.removeEventListener('focus', this._onEditorFocus)
      view.dom.removeEventListener('blur', this._onEditorBlur)
    }

    this._slashProvider?.destroy()
    this._slashProvider = null
    this._menu?.destroy()
    this._menu = null
    this._toolbar?.destroy()
    this._toolbar = null

    if (this._editor) {
      void this._editor.destroy()
      this._editor = null
    }
    this._fallbackTextarea = null
    this._placeholderEl = null
    this._menuAnchor = null
    if (this._shell && this._shell.parentNode === this) this.removeChild(this._shell)
    this._shell = null

    this._dirty = false
    this._armed = false
    this._activeMatchLength = 0
    this._dismissedQuery = null
    // Document-scoped, like the rest: a span records positions in the document
    // that produced it, so carrying it across a teardown points it into a
    // different one. The text check would usually catch that, but "usually" is
    // luck — a new body with ` @` at the same offset would lose two characters.
    this._promptedRefSpan = null
  }
}

if (!customElements.get(TAG)) {
  customElements.define(TAG, RelaEditorElement)
}
