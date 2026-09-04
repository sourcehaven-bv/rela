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

// No `etag` field here on purpose (TKT-DPBQ7S). One was declared for a long
// time and written by NO path — `fetchEntity`, `create` and `update` all
// construct `{entity, timestamp}`. A declared-but-never-written ETag is a
// trap: it invites the assumption that cached reads carry an ETag, so an
// If-Match built from this cache would send `undefined` (silently degrading
// to last-write-wins) or, worse, a stale value. The etag-bearing read path
// must go to the network — a cached entity has no meaningful ETag.
interface EntityCache {
  entity: Entity
  timestamp: number
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
  //
  // The cache key carries the WORLD, because one entity id names a different
  // FACE in each world: `POL-1` in `published` and `POL-1` in the default
  // world are different content under the same id. Keying on `type:id` alone
  // meant the first face fetched was served to every later request for the
  // other — the draft body rendered under a published-world read, or worse,
  // the published body rendered while the user edits the draft.
  //
  // The default world is spelled as the empty segment rather than the literal
  // "default" so that a key built for an unspecified world is byte-identical
  // to the pre-worlds key. `?world=default` is accepted by the API as an
  // explicit way to name the default world (dataentry/world.go), so it
  // normalizes to the same segment here — two spellings of one world must not
  // become two cache entries that can disagree.
  function worldKey(world?: string): string {
    // Verbatim, NOT normalised: on a deployment with `default_world`, an absent
    // param (the configured world) and an explicit `default` (the bare faces)
    // are two different faces of the same entity, and one cache slot for both
    // would serve one as the other. Callers pass `useWorld().worldParam`,
    // which already spells the default world the way the server reads it.
    return world ?? ''
  }

  function cacheKey(type: string, id: string, world?: string): string {
    return `${type}:${id}:${worldKey(world)}`
  }

  function listCacheKey(type: string, params?: ListParams): string {
    return `${type}:${JSON.stringify(params || {})}`
  }

  function isCacheValid(timestamp: number): boolean {
    return Date.now() - timestamp < CACHE_TTL
  }

  // Getters
  const getCached = computed(() => (type: string, id: string, world?: string) => {
    const key = cacheKey(type, id, world)
    const cached = cache.value.get(key)
    if (cached && isCacheValid(cached.timestamp)) {
      return cached.entity
    }
    return undefined
  })

  // Takes a world for the same reason getCached does: fetchEntity registers
  // its in-flight key per world, so a world-blind lookup here would report
  // "not loading" during a world-scoped fetch. The id-less form is unchanged
  // — it asks "is anything of this type loading", which spans all worlds.
  const isLoading = computed(() => (type: string, id?: string, world?: string) => {
    if (id) {
      return loading.value.has(cacheKey(type, id, world))
    }
    return Array.from(loading.value).some((k) => k.startsWith(type + ':'))
  })

  const getError = computed(() => (type: string, id: string, world?: string) => {
    return errors.value.get(cacheKey(type, id, world))
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

      // Also cache individual entities, under THIS request's world. A list
      // read under `?world=published` returns published faces, so filing them
      // under the default-world key would hand a later default-world
      // fetchEntity the published body — a cache poisoning with a wider blast
      // radius than the detail path, because merely visiting a list is enough
      // to trigger it.
      //
      // A row is also a PARTIAL entity: the list shape omits _fields,
      // _relations, _redacted and _attachments (see the v1.Entity subset in
      // internal/apiwire). That was true before worlds and is unchanged here.
      for (const entity of response.data) {
        cache.value.set(cacheKey(type, entity.id, params?.world), {
          entity,
          timestamp: Date.now(),
        })
      }

      return { data: response.data, meta: response.meta, included: response.included, _actions: response._actions }
    } finally {
      loading.value.delete(`list:${type}`)
    }
  }

  async function fetchEntity(
    type: string,
    id: string,
    force = false,
    world?: string
  ): Promise<Entity> {
    const key = cacheKey(type, id, world)

    if (!force) {
      const cached = cache.value.get(key)
      if (cached && isCacheValid(cached.timestamp)) {
        return cached.entity
      }
    }

    loading.value.add(key)
    errors.value.delete(key)

    try {
      // `include=*` fetches titles for related entities, and it goes ALONGSIDE
      // the world: neighbor resolution is world-scoped (TKT-WRLDAPI item 4),
      // so an included peer is that world's face of the neighbor. An earlier
      // revision dropped include under a world to dodge a 422 the API no
      // longer returns, and quietly lost every neighbor title with it — when
      // a backend refusal is removed, its client-side workarounds do not
      // remove themselves.
      const params: { include: string; world?: string } = { include: '*' }
      if (world) params.world = world
      const entity = await getEntity(type, id, params)
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

  // invalidateEntityWorlds drops every WORLD-SCOPED copy of one entity,
  // leaving the default-world entry the caller is about to overwrite.
  //
  // The API refuses `?world=` on a write (422 world_read_only), and the SPA's
  // edit affordances address the bare id, so a write from here updates the
  // default face.
  // But a world-scoped copy of the same id may still be cached, and it is now
  // of unknown freshness: whether it moved depends on the world's fallback
  // rule. Under `otherwise: default`, editing the default face CHANGES what
  // `site-nl` serves for an entity with no Dutch face. Re-deriving which
  // worlds fall through to the edited face would mean teaching the SPA the
  // resolution rules; dropping them is cheap, and the next read refetches.
  //
  // Keyed on the BARE id: a write addressed to `POL-1@published` changes what
  // every world resolving to that face serves for `POL-1`, so the faced and
  // the bare entries alike are dropped — every spelling of this entity
  // except the one the caller is about to overwrite.
  function invalidateEntityWorlds(type: string, id: string) {
    const defaultKey = cacheKey(type, id)
    const bare = id.includes('@') ? id.slice(0, id.indexOf('@')) : id
    const prefixes = [`${type}:${bare}:`, `${type}:${bare}@`]
    for (const key of cache.value.keys()) {
      if (key !== defaultKey && prefixes.some((p) => key.startsWith(p))) {
        cache.value.delete(key)
      }
    }
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
    invalidateEntityWorlds(type, id)
    // Invalidate list cache for this type
    invalidateListCache(type)
    // Refresh git status (non-blocking)
    refreshGitStatus()
    return entity
  }

  async function remove(type: string, id: string): Promise<void> {
    await deleteEntity(type, id)
    cache.value.delete(cacheKey(type, id))
    invalidateEntityWorlds(type, id)
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
