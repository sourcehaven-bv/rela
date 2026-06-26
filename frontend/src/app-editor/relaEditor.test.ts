import { describe, it, expect, vi, beforeEach } from 'vitest'

// The <rela-editor> element wraps EasyMDE, which needs real layout (CodeMirror 5)
// and doesn't mount under happy-dom. We mock EasyMDE so these tests exercise the
// ELEMENT's public contract — the swap seam (value/placeholder/readonly/events/
// focus/teardown) — independent of the editor implementation. A real-browser
// mount is covered by the manual app smoke test; here we pin the contract.

interface FakeCM {
  on(ev: string, fn: () => void): void
  off(ev: string, fn: () => void): void
  focus: ReturnType<typeof vi.fn>
  setOption: ReturnType<typeof vi.fn>
  _emit(ev: string): void
}

interface FakeEasyMDE {
  codemirror: FakeCM
  value(v?: string): string
  toTextArea: ReturnType<typeof vi.fn>
  cleanup: ReturnType<typeof vi.fn>
}

let lastEditor: FakeEasyMDE | null = null

// Built with closures (no `this`) so the handler registry and value are
// captured locally — keeps eslint's no-this-alias happy and the fake simple.
function makeFakeEasyMDE(opts: { initialValue?: string }): FakeEasyMDE {
  let val = opts.initialValue ?? ''
  const handlers: Record<string, Array<() => void>> = {}
  const cm: FakeCM = {
    on: (ev, fn) => {
      ;(handlers[ev] ||= []).push(fn)
    },
    off: (ev, fn) => {
      handlers[ev] = (handlers[ev] || []).filter((f) => f !== fn)
    },
    focus: vi.fn(),
    setOption: vi.fn(),
    _emit: (ev) => {
      ;(handlers[ev] || []).forEach((f) => f())
    },
  }
  const editor: FakeEasyMDE = {
    codemirror: cm,
    value: (v?: string) => {
      if (v !== undefined) val = v
      return val
    },
    toTextArea: vi.fn(),
    cleanup: vi.fn(),
  }
  lastEditor = editor
  return editor
}

// EasyMDE is used as `new EasyMDE(opts)`; wrap the factory so `new` works.
const FakeEasyMDE = function (this: unknown, opts: { initialValue?: string }) {
  return makeFakeEasyMDE(opts)
} as unknown as new (opts: { initialValue?: string }) => FakeEasyMDE

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

describe('<rela-editor> contract', () => {
  beforeEach(() => {
    lastEditor = null
    document.body.replaceChildren()
  })

  it('registers the custom element', () => {
    expect(customElements.get('rela-editor')).toBeTruthy()
  })

  it('flushes a value set BEFORE connection into the editor', () => {
    const ed = makeEditor()
    ed.value = '# Before connect'
    document.body.appendChild(ed)
    expect(ed.value).toBe('# Before connect')
    expect(lastEditor!.value()).toBe('# Before connect')
  })

  it('round-trips value set AFTER connection', () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    ed.value = '## After'
    expect(ed.value).toBe('## After')
  })

  it('preserves whitespace-sensitive markdown exactly', () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    const md = '- a\n    indented code\n\n\ttab line\n'
    ed.value = md
    expect(ed.value).toBe(md)
  })

  it('dispatches input on CodeMirror change and change on blur', () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    const onInput = vi.fn()
    const onChange = vi.fn()
    ed.addEventListener('input', onInput)
    ed.addEventListener('change', onChange)
    lastEditor!.codemirror._emit('change')
    lastEditor!.codemirror._emit('blur')
    expect(onInput).toHaveBeenCalledOnce()
    expect(onChange).toHaveBeenCalledOnce()
  })

  it('applies readonly via attribute at mount and on change', () => {
    const ed = makeEditor()
    ed.setAttribute('readonly', '')
    document.body.appendChild(ed)
    expect(lastEditor!.codemirror.setOption).toHaveBeenCalledWith('readOnly', true)
    ed.removeAttribute('readonly')
    expect(lastEditor!.codemirror.setOption).toHaveBeenCalledWith('readOnly', false)
  })

  it('focus() delegates to the editor', () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    ed.focus()
    expect(lastEditor!.codemirror.focus).toHaveBeenCalledOnce()
  })

  it('tears down EasyMDE on disconnect and preserves value across re-connect', () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    ed.value = 'keep me'
    const first = lastEditor!
    ed.remove()
    expect(first.toTextArea).toHaveBeenCalledOnce()
    expect(first.cleanup).toHaveBeenCalledOnce()
    // Re-connect: a fresh editor seeded with the preserved value.
    document.body.appendChild(ed)
    expect(ed.value).toBe('keep me')
    expect(lastEditor!.value()).toBe('keep me')
  })
})
