import axios from 'axios'
import { vi } from 'vitest'
import { config } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import axios from 'axios'

// BUG-2OXEW0: no unit test may issue a real HTTP request.
//
// The failure this prevents is indirect and badly attributed. An unstubbed
// child that fetches in `onMounted` (SidePanel, ExportMenu, ...) issues a live
// request against happy-dom's default origin. It fails ECONNREFUSED and the
// component logs it via console.error. If that log settles AFTER its test file
// finishes, it races vitest's closing `onUserConsoleLog` RPC and the run dies
// with `EnvironmentTeardownError` — exit 1, every test passing, naming a file
// the PR author never touched. It surfaced on a docs-only PR.
//
// Unmounting does not help: it stops the component but does not cancel an
// in-flight request, so the rejection still settles and still logs.
//
// Stub lists alone cannot fix this. They are a per-file denylist maintained by
// whoever last read a stack trace, so they only ever cover the file that
// happened to lose the race — the first fix for this bug stubbed SidePanel in
// the one named file and left 15 live requests in EntityList.test.ts.
//
// So fail closed here instead. The adapter rejects SYNCHRONOUSLY at request
// time, which is the point: the error is thrown at the call site, in the test
// that caused it, instead of arriving as a nondeterministic teardown crash.
// Note axios uses the Node http adapter under happy-dom, not XHR — patching
// `fetch` or XMLHttpRequest catches nothing.
//
// To let a test make a request, mock the module it calls (`vi.mock('@/api')`)
// or stub the component that renders it. Do not disable this guard.
axios.defaults.adapter = (cfg) => {
  const url = `${cfg.baseURL ?? ''}${cfg.url ?? ''}`
  throw new Error(
    `BUG-2OXEW0: unmocked HTTP request in a unit test: ${(cfg.method ?? 'get').toUpperCase()} ${url}\n` +
      `Stub the component that fetches it, or vi.mock() the api module it calls.`
  )
}

// Mock localStorage before any imports that might use it
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = value
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
    get length() {
      return Object.keys(store).length
    },
    key: (index: number) => Object.keys(store)[index] ?? null,
  }
})()
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock })

// Reset Pinia before each test
beforeEach(() => {
  setActivePinia(createPinia())
})

// Mock crypto.randomUUID for toast IDs
Object.defineProperty(globalThis, 'crypto', {
  value: {
    randomUUID: () => Math.random().toString(36).substring(2, 15),
  },
})

// Global stubs for router-link and router-view
config.global.stubs = {
  RouterLink: {
    template: '<a><slot /></a>',
    props: ['to'],
  },
  RouterView: true,
}

// Mock ResizeObserver
vi.stubGlobal(
  'ResizeObserver',
  vi.fn(() => ({
    observe: vi.fn(),
    unobserve: vi.fn(),
    disconnect: vi.fn(),
  }))
)

// Mock EventSource for SSE tests
class MockEventSource {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 2

  url: string
  readyState = MockEventSource.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  private listeners: Map<string, ((event: MessageEvent) => void)[]> = new Map()

  constructor(url: string) {
    this.url = url
    // Simulate connection on next tick
    setTimeout(() => {
      this.readyState = MockEventSource.OPEN
      this.onopen?.(new Event('open'))
    }, 0)
  }

  addEventListener(type: string, listener: (event: MessageEvent) => void) {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, [])
    }
    this.listeners.get(type)!.push(listener)
  }

  removeEventListener(type: string, listener: (event: MessageEvent) => void) {
    const typeListeners = this.listeners.get(type)
    if (typeListeners) {
      const index = typeListeners.indexOf(listener)
      if (index !== -1) {
        typeListeners.splice(index, 1)
      }
    }
  }

  close() {
    this.readyState = MockEventSource.CLOSED
  }

  // Test helper: emit an event
  _emit(type: string, data?: string) {
    const event = new MessageEvent(type, { data })
    const typeListeners = this.listeners.get(type)
    if (typeListeners) {
      typeListeners.forEach((listener) => listener(event))
    }
  }

  // Test helper: simulate error
  _error() {
    this.onerror?.(new Event('error'))
  }
}

vi.stubGlobal('EventSource', MockEventSource)

// ---------------------------------------------------------------------------
// Block real HTTP from the unit suite.
//
// Every component that mounts an ExportMenu (transitively: DynamicForm,
// EntityDetail, list views) calls getTransforms() in onMounted. Nothing stubbed
// the HTTP layer, so those calls left the process and tried to reach
// localhost:3000. Locally that fails immediately with ECONNREFUSED and the
// component's own catch swallows it; on a slower, more contended CI runner the
// socket is torn down mid-request instead, and the rejection lands AFTER the
// test that triggered it has finished — so no test owns it and vitest reports
// "Unhandled Rejection: socket hang up" and exits 1 with every test passing.
// That is the intermittent Frontend failure.
//
// Replacing the adapter fixes the class, not the instance: any axios call from
// any test resolves in-process, deterministically, with no socket. A test that
// wants a specific payload still mocks its own module (vi.mock) as before —
// this only decides what an UNMOCKED call does, and "empty 200" keeps the
// mount-and-assert tests that never cared about the response working unchanged.
//
// Deliberately NOT a rejection: several components log to console.error on a
// failed load, which would spam every unrelated test's output.
axios.defaults.adapter = async (config) => ({
  data: [],
  status: 200,
  statusText: 'OK',
  headers: {},
  config,
})
