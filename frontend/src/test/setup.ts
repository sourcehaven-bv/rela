import { vi } from 'vitest'
import { config } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import axios from 'axios'

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
//
// BUG-2OXEW0 hit the same class from the other end (SidePanel, not ExportMenu)
// and is worth recording here so the next person does not re-litigate it:
// per-file `stubs: { ... }` lists CANNOT fix this. A stub list is a denylist
// maintained by whoever last read a stack trace, so it only ever covers the
// file that happened to lose the race — stubbing the one file named in a
// traceback left 15 live requests in EntityList.test.ts. The adapter is the
// enforcing mechanism; stubs are hygiene.
axios.defaults.adapter = async (config) => ({
  data: [],
  status: 200,
  statusText: 'OK',
  headers: {},
  config,
})
