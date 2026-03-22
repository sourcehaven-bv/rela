import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useEntitiesStore } from './entities'
import * as entitiesApi from '@/api/entities'
import type { Entity } from '@/types'

vi.mock('@/api/entities', () => ({
  listEntities: vi.fn(),
  getEntity: vi.fn(),
  createEntity: vi.fn(),
  updateEntity: vi.fn(),
  deleteEntity: vi.fn(),
}))

// Mock git store separately to avoid issues
vi.mock('./git', () => ({
  useGitStore: () => ({
    fetchStatus: vi.fn().mockResolvedValue(undefined),
  }),
}))

describe('Entities Store', () => {
  let store: ReturnType<typeof useEntitiesStore>

  beforeEach(() => {
    setActivePinia(createPinia())
    store = useEntitiesStore()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('initial state', () => {
    it('starts with empty caches', () => {
      expect(store.cache.size).toBe(0)
      expect(store.loading.size).toBe(0)
      expect(store.errors.size).toBe(0)
    })
  })

  describe('fetchEntity', () => {
    const mockEntity = {
      id: 'TKT-001',
      type: 'ticket',
      properties: { title: 'Test Ticket', status: 'open' },
      relations: {},
    }

    it('fetches and caches entity', async () => {
      vi.mocked(entitiesApi.getEntity).mockResolvedValue(mockEntity)

      const result = await store.fetchEntity('ticket', 'TKT-001')

      expect(entitiesApi.getEntity).toHaveBeenCalledWith('ticket', 'TKT-001', { include: '*' })
      expect(result).toEqual(mockEntity)
      expect(store.getCached('ticket', 'TKT-001')).toEqual(mockEntity)
    })

    it('returns cached entity if valid', async () => {
      vi.mocked(entitiesApi.getEntity).mockResolvedValue(mockEntity)

      // First fetch
      await store.fetchEntity('ticket', 'TKT-001')
      expect(entitiesApi.getEntity).toHaveBeenCalledTimes(1)

      // Second fetch should use cache
      const cached = await store.fetchEntity('ticket', 'TKT-001')
      expect(entitiesApi.getEntity).toHaveBeenCalledTimes(1)
      expect(cached).toEqual(mockEntity)
    })

    it('bypasses cache when force=true', async () => {
      vi.mocked(entitiesApi.getEntity).mockResolvedValue(mockEntity)

      await store.fetchEntity('ticket', 'TKT-001')
      await store.fetchEntity('ticket', 'TKT-001', true)

      expect(entitiesApi.getEntity).toHaveBeenCalledTimes(2)
    })

    it('sets loading state during fetch', async () => {
      let resolvePromise: (value: typeof mockEntity) => void
      vi.mocked(entitiesApi.getEntity).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = resolve
        })
      )

      const fetchPromise = store.fetchEntity('ticket', 'TKT-001')
      expect(store.isLoading('ticket', 'TKT-001')).toBe(true)

      resolvePromise!(mockEntity)
      await fetchPromise

      expect(store.isLoading('ticket', 'TKT-001')).toBe(false)
    })

    it('stores error on fetch failure', async () => {
      vi.mocked(entitiesApi.getEntity).mockRejectedValue(new Error('Not found'))

      await expect(store.fetchEntity('ticket', 'TKT-999')).rejects.toThrow('Not found')

      expect(store.getError('ticket', 'TKT-999')).toBe('Not found')
    })

    it('clears error on successful fetch', async () => {
      vi.mocked(entitiesApi.getEntity)
        .mockRejectedValueOnce(new Error('Network error'))
        .mockResolvedValueOnce(mockEntity)

      await expect(store.fetchEntity('ticket', 'TKT-001')).rejects.toThrow()
      expect(store.getError('ticket', 'TKT-001')).toBe('Network error')

      await store.fetchEntity('ticket', 'TKT-001', true)
      expect(store.getError('ticket', 'TKT-001')).toBeUndefined()
    })
  })

  describe('fetchList', () => {
    const mockResponse = {
      data: [
        { id: 'TKT-001', type: 'ticket', properties: { title: 'First' }, relations: {} },
        { id: 'TKT-002', type: 'ticket', properties: { title: 'Second' }, relations: {} },
      ],
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    }

    it('fetches list and caches entities', async () => {
      vi.mocked(entitiesApi.listEntities).mockResolvedValue(mockResponse)

      const result = await store.fetchList('ticket')

      expect(entitiesApi.listEntities).toHaveBeenCalledWith('ticket', undefined)
      expect(result.data).toHaveLength(2)
      expect(result.meta.total).toBe(2)

      // Individual entities should be cached
      expect(store.getCached('ticket', 'TKT-001')).toBeDefined()
      expect(store.getCached('ticket', 'TKT-002')).toBeDefined()
    })

    it('uses list cache on subsequent requests', async () => {
      vi.mocked(entitiesApi.listEntities).mockResolvedValue(mockResponse)

      await store.fetchList('ticket')
      await store.fetchList('ticket')

      expect(entitiesApi.listEntities).toHaveBeenCalledTimes(1)
    })

    it('respects params in cache key', async () => {
      vi.mocked(entitiesApi.listEntities).mockResolvedValue(mockResponse)

      await store.fetchList('ticket', { 'filter[status]': 'open' })
      await store.fetchList('ticket', { 'filter[status]': 'closed' })

      expect(entitiesApi.listEntities).toHaveBeenCalledTimes(2)
    })
  })

  describe('create', () => {
    const newEntity = {
      id: 'TKT-003',
      type: 'ticket',
      properties: { title: 'New Ticket' },
      relations: {},
    }

    it('creates entity and caches it', async () => {
      vi.mocked(entitiesApi.createEntity).mockResolvedValue(newEntity)

      const result = await store.create('ticket', { properties: { title: 'New Ticket' } })

      expect(entitiesApi.createEntity).toHaveBeenCalledWith('ticket', {
        properties: { title: 'New Ticket' },
      })
      expect(result).toEqual(newEntity)
      expect(store.getCached('ticket', 'TKT-003')).toEqual(newEntity)
    })
  })

  describe('update', () => {
    const updatedEntity = {
      id: 'TKT-001',
      type: 'ticket',
      properties: { title: 'Updated Title', status: 'closed' },
      relations: {},
    }

    it('updates entity and refreshes cache', async () => {
      vi.mocked(entitiesApi.updateEntity).mockResolvedValue(updatedEntity)

      const result = await store.update('ticket', 'TKT-001', { properties: { title: 'Updated Title' } })

      expect(entitiesApi.updateEntity).toHaveBeenCalledWith(
        'ticket',
        'TKT-001',
        { properties: { title: 'Updated Title' } },
        undefined
      )
      expect(result).toEqual(updatedEntity)
      expect(store.getCached('ticket', 'TKT-001')).toEqual(updatedEntity)
    })

    it('passes etag for optimistic locking', async () => {
      vi.mocked(entitiesApi.updateEntity).mockResolvedValue(updatedEntity)

      await store.update('ticket', 'TKT-001', { properties: { title: 'Updated' } }, 'etag-123')

      expect(entitiesApi.updateEntity).toHaveBeenCalledWith(
        'ticket',
        'TKT-001',
        { properties: { title: 'Updated' } },
        'etag-123'
      )
    })
  })

  describe('remove', () => {
    it('deletes entity and clears cache', async () => {
      vi.mocked(entitiesApi.deleteEntity).mockResolvedValue(undefined)
      vi.mocked(entitiesApi.getEntity).mockResolvedValue({
        id: 'TKT-001',
        type: 'ticket',
        properties: {},
        relations: {},
      })

      // First, cache the entity
      await store.fetchEntity('ticket', 'TKT-001')
      expect(store.getCached('ticket', 'TKT-001')).toBeDefined()

      // Then delete it
      await store.remove('ticket', 'TKT-001')

      expect(entitiesApi.deleteEntity).toHaveBeenCalledWith('ticket', 'TKT-001')
      expect(store.getCached('ticket', 'TKT-001')).toBeUndefined()
    })
  })

  describe('invalidateAll', () => {
    it('clears all caches', async () => {
      vi.mocked(entitiesApi.listEntities).mockResolvedValue({
        data: [{ id: 'TKT-001', type: 'ticket', properties: {}, relations: {} }],
        meta: { total: 1, page: 1, per_page: 20, has_more: false },
      })

      await store.fetchList('ticket')
      expect(store.getCached('ticket', 'TKT-001')).toBeDefined()

      store.invalidateAll()

      expect(store.getCached('ticket', 'TKT-001')).toBeUndefined()
    })
  })

  describe('isLoading', () => {
    it('checks loading state by type without id', async () => {
      let resolvePromise: (value: Entity) => void
      vi.mocked(entitiesApi.getEntity).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = resolve
        })
      )

      const fetchPromise = store.fetchEntity('ticket', 'TKT-001')

      // Check loading by type only (no id)
      expect(store.isLoading('ticket')).toBe(true)
      expect(store.isLoading('bug')).toBe(false)

      resolvePromise!({ id: 'TKT-001', type: 'ticket', properties: {}, relations: {} })
      await fetchPromise

      expect(store.isLoading('ticket')).toBe(false)
    })
  })

  describe('list cache invalidation on mutations', () => {
    it('invalidates list cache when creating entity', async () => {
      vi.mocked(entitiesApi.listEntities).mockResolvedValue({
        data: [{ id: 'TKT-001', type: 'ticket', properties: {}, relations: {} }],
        meta: { total: 1, page: 1, per_page: 20, has_more: false },
      })
      vi.mocked(entitiesApi.createEntity).mockResolvedValue({
        id: 'TKT-002',
        type: 'ticket',
        properties: { title: 'New' },
        relations: {},
      })

      // Cache the list
      await store.fetchList('ticket')
      expect(entitiesApi.listEntities).toHaveBeenCalledTimes(1)

      // Create should invalidate list cache
      await store.create('ticket', { properties: { title: 'New' } })

      // Fetching list again should call API (cache was invalidated)
      await store.fetchList('ticket')
      expect(entitiesApi.listEntities).toHaveBeenCalledTimes(2)
    })
  })

  describe('cache expiration', () => {
    it('expires cache after TTL', async () => {
      vi.useFakeTimers()

      const mockEntity = {
        id: 'TKT-001',
        type: 'ticket',
        properties: {},
        relations: {},
      }
      vi.mocked(entitiesApi.getEntity).mockResolvedValue(mockEntity)

      await store.fetchEntity('ticket', 'TKT-001')
      expect(store.getCached('ticket', 'TKT-001')).toBeDefined()

      // Advance time past cache TTL (60 seconds)
      vi.advanceTimersByTime(61000)

      expect(store.getCached('ticket', 'TKT-001')).toBeUndefined()
    })
  })
})
