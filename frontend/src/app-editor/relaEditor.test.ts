import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest'

// The <rela-editor> element wraps EasyMDE, which needs real layout (CodeMirror 5)
// and doesn't mount under happy-dom. We mock EasyMDE so these tests exercise the
// ELEMENT's public contract — the swap seam (value/placeholder/readonly/events/
// focus/teardown) — independent of the editor implementation. A real-browser
// mount is covered by the manual app smoke test; here we pin the contract.

interface FakeCM {
  on(ev: string, fn: () => void): void
  off(ev: string, fn: () => void): void
  focus: ReturnType<typeof vi.fn>
  setOption: Mock
  getOption(name: string): unknown
  _emit(ev: string): void
  _setValue(v: string): void // simulate a USER edit (no setter suppression)
  _options: Record<string, unknown>
}

interface FakeEasyMDE {
  codemirror: FakeCM
  value(v?: string): string
  toTextArea: ReturnType<typeof vi.fn>
  cleanup: ReturnType<typeof vi.fn>
}

let lastEditor: FakeEasyMDE | null = null
const toggleFullScreenSpy = vi.fn()

// Built with closures (no `this`) so the handler registry and value are
// captured locally — keeps eslint's no-this-alias happy and the fake simple.
function makeFakeEasyMDE(opts: { initialValue?: string }): FakeEasyMDE {
  let val = opts.initialValue ?? ''
  const handlers: Record<string, Array<() => void>> = {}
  const options: Record<string, unknown> = {}
  const cm: FakeCM = {
    on: (ev, fn) => {
      ;(handlers[ev] ||= []).push(fn)
    },
    off: (ev, fn) => {
      handlers[ev] = (handlers[ev] || []).filter((f) => f !== fn)
    },
    focus: vi.fn(),
    setOption: vi.fn((name: string, v: unknown) => {
      options[name] = v
    }),
    getOption: (name: string) => options[name],
    _emit: (ev) => {
      ;(handlers[ev] || []).forEach((f) => f())
    },
    _setValue: (v: string) => {
      val = v
      cm._emit('change')
    },
    _options: options,
  }
  const editor: FakeEasyMDE = {
    codemirror: cm,
    value: (v?: string) => {
      if (v !== undefined) {
        val = v
        // Real EasyMDE.value(v) drives CodeMirror's 'change' handler; mirror
        // that so the element's programmatic-set suppression is exercised.
        cm._emit('change')
      }
      return val
    },
    toTextArea: vi.fn(),
    cleanup: vi.fn(),
  }
  lastEditor = editor
  return editor
}

// EasyMDE is used as `new EasyMDE(opts)` with a static `toggleFullScreen`.
const FakeEasyMDE = function (this: unknown, opts: { initialValue?: string }) {
  return makeFakeEasyMDE(opts)
} as unknown as (new (opts: { initialValue?: string }) => FakeEasyMDE) & {
  toggleFullScreen: (e: FakeEasyMDE) => void
}
FakeEasyMDE.toggleFullScreen = (e: FakeEasyMDE) => {
  toggleFullScreenSpy(e)
  // mirror EasyMDE: flips the cm fullScreen option
  e.codemirror.setOption('fullScreen', !e.codemirror.getOption('fullScreen'))
}

vi.mock('easymde', () => ({ default: FakeEasyMDE }))
// The ?inline CSS imports return strings; stub them so the module loads.
vi.mock('easymde/dist/easymde.min.css?inline', () => ({ default: '' }))
vi.mock('font-awesome/css/font-awesome.min.css?inline', () => ({ default: '' }))
vi.mock('./relaEditorFont.css?inline', () => ({ default: '' }))
vi.mock('./relaEditorTheme.css?inline', () => ({ default: '' }))

// Import after mocks so customElements.define runs against the fake.
await import('./relaEditor')

function makeEditor(): HTMLElement & { value: string } {
  return document.createElement('rela-editor') as HTMLElement & { value: string }
}

// connectedCallback defers the EasyMDE mount to a microtask (so it doesn't
// block the parse path). flush() awaits that microtask so tests observe the
// mounted editor.
const flush = () => Promise.resolve()

describe('<rela-editor> contract', () => {
  beforeEach(() => {
    lastEditor = null
    toggleFullScreenSpy.mockClear()
    document.body.replaceChildren()
  })

  it('registers the custom element', () => {
    expect(customElements.get('rela-editor')).toBeTruthy()
  })

  it('flushes a value set BEFORE connection into the editor', async () => {
    const ed = makeEditor()
    ed.value = '# Before connect'
    document.body.appendChild(ed)
    await flush()
    expect(ed.value).toBe('# Before connect')
    expect(lastEditor!.value()).toBe('# Before connect')
  })

  it('round-trips value set AFTER connection', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    ed.value = '## After'
    expect(ed.value).toBe('## After')
  })

  it('preserves whitespace-sensitive markdown exactly', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    const md = '- a\n    indented code\n\n\ttab line\n'
    ed.value = md
    expect(ed.value).toBe(md)
  })

  it('does NOT emit input when value is set programmatically', async () => {
    // A native <textarea>.value setter is silent; <rela-editor> must match,
    // else loading content into the editor triggers autosave loops in
    // consumers (the bug that froze the Today app's goals load). The fake's
    // value() emits a CodeMirror 'change' like EasyMDE, so the suppression
    // path is exercised.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    const onInput = vi.fn()
    ed.addEventListener('input', onInput)
    ed.value = 'programmatic content'
    expect(onInput).not.toHaveBeenCalled()
    // But a genuine user edit (cm change OUTSIDE the setter) still fires input.
    lastEditor!.codemirror._emit('change')
    expect(onInput).toHaveBeenCalledOnce()
  })

  it('dispatches input on CodeMirror change, and change on blur after an edit', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    const onInput = vi.fn()
    const onChange = vi.fn()
    ed.addEventListener('input', onInput)
    ed.addEventListener('change', onChange)
    lastEditor!.codemirror._emit('focus') // snapshot value at focus
    lastEditor!.codemirror._setValue('user typed this') // a real edit
    lastEditor!.codemirror._emit('blur')
    expect(onInput).toHaveBeenCalledOnce()
    expect(onChange).toHaveBeenCalledOnce()
  })

  it('does NOT dispatch change on blur when nothing was edited (native textarea semantics)', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    const onChange = vi.fn()
    ed.addEventListener('change', onChange)
    // Focus then blur with no edit in between → no change event (a click-away
    // must not trigger autosave).
    lastEditor!.codemirror._emit('focus')
    lastEditor!.codemirror._emit('blur')
    expect(onChange).not.toHaveBeenCalled()
  })

  it('applies readonly via attribute at mount and on change', async () => {
    const ed = makeEditor()
    ed.setAttribute('readonly', '')
    document.body.appendChild(ed)
    await flush()
    expect(lastEditor!.codemirror.setOption).toHaveBeenCalledWith('readOnly', true)
    ed.removeAttribute('readonly')
    expect(lastEditor!.codemirror.setOption).toHaveBeenCalledWith('readOnly', false)
  })

  it('focus() delegates to the editor', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    ed.focus()
    expect(lastEditor!.codemirror.focus).toHaveBeenCalledOnce()
  })

  it('tears down EasyMDE on disconnect and preserves value across re-connect', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    ed.value = 'keep me'
    const first = lastEditor!
    ed.remove()
    expect(first.toTextArea).toHaveBeenCalledOnce()
    expect(first.cleanup).toHaveBeenCalledOnce()
    // Re-connect: a fresh editor seeded with the preserved value.
    document.body.appendChild(ed)
    await flush()
    expect(ed.value).toBe('keep me')
    expect(lastEditor!.value()).toBe('keep me')
  })

  it('exits fullscreen on teardown so the host page is not left unscrollable', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    // Simulate the user entering fullscreen (EasyMDE sets the cm option).
    lastEditor!.codemirror.setOption('fullScreen', true)
    ed.remove()
    // Teardown must have toggled fullscreen back off (which restores
    // document.body.style.overflow in real EasyMDE).
    expect(toggleFullScreenSpy).toHaveBeenCalledOnce()
  })

  it('does not toggle fullscreen on teardown when not fullscreen', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await flush()
    ed.remove()
    expect(toggleFullScreenSpy).not.toHaveBeenCalled()
  })
})
