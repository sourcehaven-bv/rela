import { vi } from 'vitest'
import { config } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

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
