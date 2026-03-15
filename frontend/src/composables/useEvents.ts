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
      // For now, we invalidate all caches on any entity change.
      // Future: use the event data to invalidate only affected caches.
      eventSource.addEventListener('entity:created', (event: MessageEvent) => {
        try {
          JSON.parse(event.data) as EntityEventData // validate format
          entitiesStore.invalidateAll()
        } catch {
          console.warn('Failed to parse entity:created event data')
        }
      })

      eventSource.addEventListener('entity:updated', (event: MessageEvent) => {
        try {
          JSON.parse(event.data) as EntityEventData
          entitiesStore.invalidateAll()
        } catch {
          console.warn('Failed to parse entity:updated event data')
        }
      })

      eventSource.addEventListener('entity:deleted', (event: MessageEvent) => {
        try {
          JSON.parse(event.data) as EntityEventData
          entitiesStore.invalidateAll()
        } catch {
          console.warn('Failed to parse entity:deleted event data')
        }
      })
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

  return {
    connectionState,
    connect,
    disconnect,
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
