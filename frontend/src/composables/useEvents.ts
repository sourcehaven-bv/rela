import { ref, onMounted, onUnmounted } from 'vue'
import { useGitStore, useEntitiesStore } from '@/stores'

export type SSEEventType =
  | 'refresh'
  | 'git'
  | 'git:status'
  | 'entity:created'
  | 'entity:updated'
  | 'entity:deleted'

export interface EntityEventData {
  type: string
  id: string
}

export interface SSEConnectionState {
  connected: boolean
  reconnecting: boolean
  error: string | null
}

// Singleton state - shared across all components
const connectionState = ref<SSEConnectionState>({
  connected: false,
  reconnecting: false,
  error: null,
})

let eventSource: EventSource | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let reconnectAttempts = 0
const MAX_RECONNECT_DELAY = 30000 // 30 seconds max
const BASE_RECONNECT_DELAY = 1000 // 1 second base

// Custom event handlers registry
type EventHandler = (data: EntityEventData) => void
const eventHandlers: Map<SSEEventType, Set<EventHandler>> = new Map()

/**
 * Composable for Server-Sent Events from the backend.
 * Handles connection lifecycle, auto-reconnection, and event dispatching to stores.
 *
 * Events:
 * - refresh: Files changed, full reload needed
 * - git / git:status: Git status changed
 * - entity:created: Entity created (data: {type, id})
 * - entity:updated: Entity updated (data: {type, id})
 * - entity:deleted: Entity deleted (data: {type, id})
 */
export function useEvents() {
  const gitStore = useGitStore()
  const entitiesStore = useEntitiesStore()

  function getReconnectDelay(): number {
    // Exponential backoff: 1s, 2s, 4s, 8s, ... up to 30s
    const delay = Math.min(BASE_RECONNECT_DELAY * Math.pow(2, reconnectAttempts), MAX_RECONNECT_DELAY)
    return delay
  }

  function connect() {
    if (eventSource) {
      return // Already connected
    }

    // Clear any pending reconnect
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }

    connectionState.value = {
      connected: false,
      reconnecting: reconnectAttempts > 0,
      error: null,
    }

    try {
      eventSource = new EventSource('/api/v1/_events')

      eventSource.onopen = () => {
        reconnectAttempts = 0
        connectionState.value = {
          connected: true,
          reconnecting: false,
          error: null,
        }
      }

      eventSource.onerror = () => {
        // EventSource will automatically try to reconnect,
        // but we handle it manually for better control
        disconnect()
        scheduleReconnect()
      }

      // Handle refresh event (full reload)
      eventSource.addEventListener('refresh', () => {
        // Invalidate all caches and refetch
        entitiesStore.invalidateAll()
        gitStore.fetchStatus().catch(() => {})
      })

      // Handle git events
      eventSource.addEventListener('git', () => {
        gitStore.fetchStatus().catch(() => {})
      })

      eventSource.addEventListener('git:status', () => {
        gitStore.fetchStatus().catch(() => {})
      })

      // Handle entity events
      // Invalidate caches and dispatch to custom handlers
      const entityEventTypes: SSEEventType[] = ['entity:created', 'entity:updated', 'entity:deleted']
      for (const eventType of entityEventTypes) {
        eventSource.addEventListener(eventType, (event: MessageEvent) => {
          try {
            const data = JSON.parse(event.data) as EntityEventData
            entitiesStore.invalidateAll()
            // Dispatch to custom handlers
            eventHandlers.get(eventType)?.forEach((handler) => handler(data))
          } catch {
            console.warn(`Failed to parse ${eventType} event data`)
          }
        })
      }
    } catch (err) {
      connectionState.value = {
        connected: false,
        reconnecting: false,
        error: err instanceof Error ? err.message : 'Connection failed',
      }
      scheduleReconnect()
    }
  }

  function disconnect() {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
    connectionState.value = {
      ...connectionState.value,
      connected: false,
    }
  }

  function scheduleReconnect() {
    if (reconnectTimer) return // Already scheduled

    reconnectAttempts++
    const delay = getReconnectDelay()

    connectionState.value = {
      connected: false,
      reconnecting: true,
      error: `Connection lost. Reconnecting in ${Math.round(delay / 1000)}s...`,
    }

    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, delay)
  }

  // Lifecycle management
  onMounted(() => {
    connect()
  })

  onUnmounted(() => {
    // Don't disconnect - keep SSE alive for other components
    // The connection is shared across the app
  })

  // Subscribe to specific event types
  function on(eventType: SSEEventType, handler: EventHandler) {
    if (!eventHandlers.has(eventType)) {
      eventHandlers.set(eventType, new Set())
    }
    eventHandlers.get(eventType)!.add(handler)
  }

  // Unsubscribe from specific event types
  function off(eventType: SSEEventType, handler: EventHandler) {
    eventHandlers.get(eventType)?.delete(handler)
  }

  return {
    connectionState,
    connect,
    disconnect,
    on,
    off,
  }
}

/**
 * Initialize SSE connection at app startup.
 * Call this once in App.vue or main.ts.
 */
export function initEvents() {
  const { connect } = useEvents()
  connect()
}
