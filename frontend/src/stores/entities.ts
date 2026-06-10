import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listEntities,
  getEntity,
  createEntity,
  updateEntity,
  deleteEntity,
  type EntityPatch,
} from '@/api/entities'
import type { Entity, CreateEntity, ListParams, ListMeta } from '@/types'
import { getErrorMessage } from '@/api/errors'
import { useGitStore } from './git'

interface EntityCache {
  entity: Entity
  timestamp: number
  etag?: string
}

const CACHE_TTL = 60 * 1000 // 1 minute

// Refresh git status after mutations (non-blocking)
function refreshGitStatus() {
  const gitStore = useGitStore()
  /* v8 ignore next 2 - best-effort error handling tested via e2e */
  gitStore.fetchStatus().catch(() => {
    // Ignore errors - git status refresh is best-effort
  })
}

export const useEntitiesStore = defineStore('entities', () => {
  // State
  const cache = ref<Map<string, EntityCache>>(new Map())
  const listCache = ref<Map<string, { data: Entity[]; meta: ListMeta; included?: Record<string, Entity>; _actions?: Record<string, boolean>; timestamp: number }>>(
    new Map()
  )
  const loading = ref<Set<string>>(new Set())
  const errors = ref<Map<string, string>>(new Map())
  const cacheVersion = ref(0) // Incremented on invalidateAll for SSE live updates

  // Helpers
  function cacheKey(type: string, id: string): string {
    return `${type}:${id}`
  }

  function listCacheKey(type: string, params?: ListParams): string {
    return `${type}:${JSON.stringify(params || {})}`
  }

  function isCacheValid(timestamp: number): boolean {
    return Date.now() - timestamp < CACHE_TTL
  }

  // Getters
  const getCached = computed(() => (type: string, id: string) => {
    const key = cacheKey(type, id)
    const cached = cache.value.get(key)
    if (cached && isCacheValid(cached.timestamp)) {
      return cached.entity
    }
    return undefined
  })

  const isLoading = computed(() => (type: string, id?: string) => {
    if (id) {
      return loading.value.has(cacheKey(type, id))
    }
    return Array.from(loading.value).some((k) => k.startsWith(type + ':'))
  })

  const getError = computed(() => (type: string, id: string) => {
    return errors.value.get(cacheKey(type, id))
  })

  // Actions
  async function fetchList(type: string, params?: ListParams): Promise<{ data: Entity[]; meta: ListMeta; included?: Record<string, Entity>; _actions?: Record<string, boolean> }> {
    const key = listCacheKey(type, params)
    const cached = listCache.value.get(key)

    if (cached && isCacheValid(cached.timestamp)) {
      return { data: cached.data, meta: cached.meta, included: cached.included, _actions: cached._actions }
    }

    loading.value.add(`list:${type}`)
    try {
      const response = await listEntities(type, params)

      // Cache list result
      listCache.value.set(key, {
        data: response.data,
        meta: response.meta,
        included: response.included,
        _actions: response._actions,
        timestamp: Date.now(),
      })

      // Also cache individual entities
      for (const entity of response.data) {
        cache.value.set(cacheKey(type, entity.id), {
          entity,
          timestamp: Date.now(),
        })
      }

      return { data: response.data, meta: response.meta, included: response.included, _actions: response._actions }
    } finally {
      loading.value.delete(`list:${type}`)
    }
  }

  async function fetchEntity(type: string, id: string, force = false): Promise<Entity> {
    const key = cacheKey(type, id)

    if (!force) {
      const cached = cache.value.get(key)
      if (cached && isCacheValid(cached.timestamp)) {
        return cached.entity
      }
    }

    loading.value.add(key)
    errors.value.delete(key)

    try {
      // Request include=* to get titles for all related entities
      const entity = await getEntity(type, id, { include: '*' })
      cache.value.set(key, {
        entity,
        timestamp: Date.now(),
      })
      return entity
    } catch (err) {
      const message = getErrorMessage(err, 'Failed to fetch entity')
      errors.value.set(key, message)
      throw err
    } finally {
      loading.value.delete(key)
    }
  }

  async function create(type: string, data: CreateEntity): Promise<Entity> {
    const entity = await createEntity(type, data)
    cache.value.set(cacheKey(type, entity.id), {
      entity,
      timestamp: Date.now(),
    })
    // Invalidate list cache for this type
    invalidateListCache(type)
    // Refresh git status (non-blocking)
    refreshGitStatus()
    return entity
  }

  async function update(
    type: string,
    id: string,
    data: EntityPatch,
    etag?: string,
    signal?: AbortSignal,
  ): Promise<Entity> {
    const entity = await updateEntity(type, id, data, etag, signal)
    cache.value.set(cacheKey(type, id), {
      entity,
      timestamp: Date.now(),
    })
    // Invalidate list cache for this type
    invalidateListCache(type)
    // Refresh git status (non-blocking)
    refreshGitStatus()
    return entity
  }

  async function remove(type: string, id: string): Promise<void> {
    await deleteEntity(type, id)
    cache.value.delete(cacheKey(type, id))
    // Invalidate list cache for this type
    invalidateListCache(type)
    // Refresh git status (non-blocking)
    refreshGitStatus()
  }

  function invalidateListCache(type: string) {
    for (const key of listCache.value.keys()) {
      if (key.startsWith(type + ':')) {
        listCache.value.delete(key)
      }
    }
  }

  function invalidateAll() {
    cache.value.clear()
    listCache.value.clear()
    cacheVersion.value++
  }

  return {
    // State
    cache,
    loading,
    errors,
    cacheVersion,

    // Getters
    getCached,
    isLoading,
    getError,

    // Actions
    fetchList,
    fetchEntity,
    create,
    update,
    remove,
    invalidateAll,
  }
})
