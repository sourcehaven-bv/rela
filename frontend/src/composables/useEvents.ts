import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useQueryCache } from '@pinia/colada'
import { useGitStore, useEntitiesStore } from '@/stores'
import { entityKeys } from '@/queries/entities'

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

/**
 * Singleton state - shared across all components.
 * This is intentional for SSE because:
 * 1. We only want one EventSource connection to the server
 * 2. Multiple components can subscribe to the same events
 * 3. Connection state should be consistent across the app
 */
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
  const queryCache = useQueryCache()

  /* v8 ignore start - reconnection logic tested via e2e */
  function getReconnectDelay(): number {
    // Exponential backoff: 1s, 2s, 4s, 8s, ... up to 30s
    const delay = Math.min(BASE_RECONNECT_DELAY * Math.pow(2, reconnectAttempts), MAX_RECONNECT_DELAY)
    return delay
  }
  /* v8 ignore stop */

  function connect() {
    if (eventSource) {
      return // Already connected
    }

    // Clear any pending reconnect
    /* v8 ignore start - reconnection logic tested via e2e */
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    /* v8 ignore stop */

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

      /* v8 ignore start - SSE error handling tested via e2e */
      eventSource.onerror = () => {
        // EventSource will automatically try to reconnect,
        // but we handle it manually for better control
        disconnect()
        scheduleReconnect()
      }
      /* v8 ignore stop */

      // Handle refresh event (full reload)
      eventSource.addEventListener('refresh', () => {
        // Invalidate all caches and refetch. invalidateAll() serves the
        // legacy entities-store TTL cache; the query-cache invalidation
        // marks every entity query stale and background-refetches the
        // active ones (FEAT-XY2D1L).
        entitiesStore.invalidateAll()
        queryCache.invalidateQueries({ key: entityKeys.root }).catch(() => {})
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
            // Legacy TTL cache (unmigrated views) is all-or-nothing.
            entitiesStore.invalidateAll()
            // The query cache uses the event's granularity: stale-mark
            // every query for this entity type (lists + details); active
            // queries background-refetch (FEAT-XY2D1L).
            queryCache
              .invalidateQueries({
                key: data.type ? entityKeys.type(data.type) : entityKeys.root,
              })
              .catch(() => {})
            // Dispatch to custom handlers
            eventHandlers.get(eventType)?.forEach((handler) => handler(data))
          } catch {
            console.warn(`Failed to parse ${eventType} event data`)
          }
        })
      }
    } catch (err) /* v8 ignore start - connection errors tested via e2e */ {
      connectionState.value = {
        connected: false,
        reconnecting: false,
        error: err instanceof Error ? err.message : 'Connection failed',
      }
      scheduleReconnect()
    } /* v8 ignore stop */
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

  /* v8 ignore start - reconnection logic tested via e2e */
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
  /* v8 ignore stop */

  // Track handlers registered by this component instance for cleanup
  const localHandlers: Array<{ type: SSEEventType; handler: EventHandler }> = []

  // Lifecycle management
  onMounted(() => {
    connect()
  })

  onBeforeUnmount(() => {
    // Clean up handlers registered by this component instance
    for (const { type, handler } of localHandlers) {
      eventHandlers.get(type)?.delete(handler)
    }
    localHandlers.length = 0
    // Don't disconnect SSE - keep alive for other components (shared connection)
  })

  // Subscribe to specific event types
  function on(eventType: SSEEventType, handler: EventHandler) {
    let handlers = eventHandlers.get(eventType)
    if (!handlers) {
      handlers = new Set()
      eventHandlers.set(eventType, handlers)
    }
    handlers.add(handler)
    // Track for cleanup on unmount
    localHandlers.push({ type: eventType, handler })
  }

  // Unsubscribe from specific event types
  function off(eventType: SSEEventType, handler: EventHandler) {
    eventHandlers.get(eventType)?.delete(handler)
    // Remove from local tracking
    const idx = localHandlers.findIndex((h) => h.type === eventType && h.handler === handler)
    if (idx >= 0) localHandlers.splice(idx, 1)
  }

  return {
    connectionState,
    connect,
    disconnect,
    on,
    off,
  }
}
