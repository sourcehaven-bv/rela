// useAutoSave: opt-in per-entity auto-save composable for data-entry forms.
//
// TKT-E6094 (this revision). Ported from the wip/autosave-TKT-18JS6 WIP
// commit with the following design-review-driven changes:
//
// * Relations channel — `scheduleRelationsChange()` marks a single
//   `relationsDirty` flag. The next debounce fire bundles relations
//   into the same PATCH (no separate request per channel). Builds the
//   body via a caller-supplied closure (`buildRelationsBody`) so the
//   composable stays Pinia-free and the form retains ownership of
//   `pendingCardChanges`.
// * Warning categorization — warnings emitted under inverse body keys
//   (TKT-GFQK's `direction: "incoming"`) are mapped back to the
//   widget-id key `${canonicalRelation}-incoming` via a caller-supplied
//   `inverseToCanonical` map.
// * `commitImmediately` returns a typed `CommitResult` and honors a
//   timeout. In-flight saves are aborted on timeout via AbortController.
// * No `If-Match` on PATCH — the FIFO chain already serializes per
//   composable instance; cross-tab conflicts resolve through the SSE
//   merge path.
// * `lastSeenServer` is only updated from server responses
//   (via `mergeServerResponse`). The WIP wrote client-sent values
//   directly, which masked server-side automation drift.

import { ref, computed, type Ref } from 'vue'
import type { Entity } from '@/types'
import type { EntityPatch } from '@/api/entities'
import { ApiError, getErrorMessage } from '@/api/errors'
import { useEntitiesStore } from '@/stores/entities'

// Sentinel for "unset this property" pending entries. Distinct from
// undefined so we can tell apart "delete the key" from "set to
// undefined" (which the API treats the same as null/"").
const UNSET = Symbol('unset')

const SAVED_INDICATOR_MS = 1200
// Minimum time the 'saving' state stays visible. Even when a PATCH
// resolves in 50ms, the indicator holds 'saving' for this long so the
// user perceives a smooth idle → saving → saved transition.
const MIN_SAVING_VISIBLE_MS = 600

export type SaveStatus = 'idle' | 'saving' | 'saved' | 'error'

export interface AutoSaveWarning {
  code: string
  path?: string
  detail?: string
  direction?: 'outgoing' | 'incoming' | string
}

// Result of commitImmediately. `settled` is true if the chain
// resolved before the timeout; `error` is non-empty when any save
// rejected. The navigation guard inspects both.
export interface CommitResult {
  settled: boolean
  error?: string
}

interface PendingEntry {
  value: unknown | typeof UNSET
  enqueuedAt: number
}

export interface AutoSaveOptions {
  getEntityType: () => string
  getEntityId: () => string
  // Legacy single debounce. When set, applies to whichever channels
  // didn't get an explicit per-channel debounce. Defaults to 800.
  debounceMs?: number
  // Per-channel debounce overrides. When omitted, fall back to
  // debounceMs. EntityDetail's content-only instance uses 100ms here
  // so checkbox toggles feel instant; DynamicForm leaves both unset
  // and inherits the legacy 800ms.
  fieldDebounceMs?: number
  contentDebounceMs?: number
  dirtyWindowMs?: number
  // Seed the lastSeenServer baseline up-front so the first edit can
  // suppress no-op writes without waiting for a server round-trip.
  // Equivalent to calling recordServerSnapshot(entity) immediately
  // after construction. Any later recordServerSnapshot call fully
  // replaces this seed.
  initialServerSnapshot?: Entity
  // Channel disable flags. When a channel is disabled:
  //   * scheduleFieldSave/scheduleUnset/scheduleContentSave/scheduleRelationsChange
  //     throws an AutoSaveChannelDisabledError on call.
  //   * mergeServerResponse still updates lastSeenServer / lastSeenContent
  //     for the disabled channel (so a future re-enable wouldn't lose
  //     the baseline) but skips the apply* callback invocation.
  //   * commitImmediately needs no special guard — disabled channels
  //     never accrue pending state.
  // Re-enabling a channel mid-instance-lifetime is explicitly not
  // supported. Spin up a new instance.
  disablePropertyChannel?: boolean
  disableContentChannel?: boolean
  disableRelationsChannel?: boolean
  // Read-only refs into the form state, used by mergeServerResponse.
  // The composable never writes to these refs — it only inspects
  // shape — so callers fabricating a computed ref (e.g. EntityDetail's
  // content-only instance) is fine.
  formData: Ref<Record<string, unknown>>
  contentRef: Ref<string>
  // Direction mapping: inverse body key → canonical relation name.
  // Used to attribute warnings on inverse-keyed paths back to the
  // widget that owns them. Empty when the form has no incoming widgets.
  inverseToCanonical: Map<string, string>
  // Closure that returns the modern relations body to attach to the
  // next PATCH, or null/empty object when the relations Map is
  // pristine. Called once per fire that has `relationsDirty === true`.
  // Callers that disable the relations channel may pass a no-op (() => null).
  buildRelationsBody: () => Record<string, { data: unknown[] }> | null
  // Apply callbacks invoked by mergeServerResponse and revertField.
  // The form decides whether to mutate formData; the composable does not.
  // Callers that disable the corresponding channel may pass a no-op closure;
  // these stay required at the type level so disabling is opt-in and
  // explicit rather than load-bearing on undefined-checks.
  applyServerProperty: (property: string, value: unknown) => void
  applyServerContent: (content: string) => void
  // User-facing error surface (e.g., toast). Called once per save
  // failure that isn't superseded by a newer edit. The structured
  // `info` carries the HTTP status, the failing property (when
  // applicable), and the channel that originated the failure so a
  // host can dispatch on 401/403 distinctly from validation errors.
  // The arg is optional so existing callers (`(msg) => uiStore.error(msg)`)
  // ignore it silently. Set per call site inside the composable.
  onError: (msg: string, info?: AutoSaveErrorInfo) => void
}

export interface AutoSaveErrorInfo {
  status?: number
  property?: string
  channel?: 'property' | 'content' | 'relations'
}

export class AutoSaveChannelDisabledError extends Error {
  constructor(channel: 'property' | 'content' | 'relations') {
    super(`useAutoSave: ${channel} channel is disabled on this instance`)
    this.name = 'AutoSaveChannelDisabledError'
  }
}

type WidgetId = `${string}-outgoing` | `${string}-incoming`

export function useAutoSave(opts: AutoSaveOptions) {
  const baseDebounceMs = opts.debounceMs ?? 800
  const fieldDebounceMs = opts.fieldDebounceMs ?? baseDebounceMs
  const contentDebounceMs = opts.contentDebounceMs ?? baseDebounceMs
  const relationsDebounceMs = baseDebounceMs
  const dirtyWindowMs = opts.dirtyWindowMs ?? 1500
  const propertyChannelEnabled = !opts.disablePropertyChannel
  const contentChannelEnabled = !opts.disableContentChannel
  const relationsChannelEnabled = !opts.disableRelationsChannel
  const entitiesStore = useEntitiesStore()

  const status = ref<SaveStatus>('idle')
  const lastError = ref<string | null>(null)
  const inFlightCount = ref(0)
  const pendingCount = ref(0)
  const fieldErrors = ref<Record<string, string>>({})
  const fieldWarnings = ref<Record<string, AutoSaveWarning>>({})
  const contentError = ref<string | null>(null)
  const contentWarning = ref<AutoSaveWarning | null>(null)
  const relationWarnings = ref<Partial<Record<WidgetId, AutoSaveWarning>>>({})

  // Last-seen server value per property — used for no-op suppression.
  // Written ONLY by recordServerSnapshot and mergeServerResponse — never
  // from client-sent values (S5 design-review fix).
  const lastSeenServer: Record<string, unknown> = {}
  let lastSeenContent = ''

  const pending: Record<string, PendingEntry> = Object.create(null)
  let pendingContent: { value: string; enqueuedAt: number } | null = null
  const timers: Record<string, ReturnType<typeof setTimeout>> = Object.create(null)
  let contentTimer: ReturnType<typeof setTimeout> | null = null

  // Relations channel: a single boolean (not per-relation). The form
  // owns the Map; the composable just remembers "kick the queue on
  // next debounce fire."
  let relationsDirty = false
  let relationsTimer: ReturnType<typeof setTimeout> | null = null

  const lastCommitAt: Record<string, number> = Object.create(null)
  let queueTail: Promise<void> = Promise.resolve()

  // AbortController plumbing — used by commitImmediately on timeout.
  let currentAbort: AbortController | null = null

  let savedIndicatorTimer: ReturnType<typeof setTimeout> | null = null
  let savingStartedAt = 0
  let pendingStatusTimer: ReturnType<typeof setTimeout> | null = null

  function setStatus(next: SaveStatus, err?: string) {
    if (pendingStatusTimer) {
      clearTimeout(pendingStatusTimer)
      pendingStatusTimer = null
    }
    if (savedIndicatorTimer) {
      clearTimeout(savedIndicatorTimer)
      savedIndicatorTimer = null
    }
    if (status.value === 'saving' && next !== 'saving') {
      const elapsed = Date.now() - savingStartedAt
      const remaining = MIN_SAVING_VISIBLE_MS - elapsed
      if (remaining > 0) {
        pendingStatusTimer = setTimeout(() => {
          pendingStatusTimer = null
          applyStatus(next, err)
        }, remaining)
        return
      }
    }
    applyStatus(next, err)
  }

  function applyStatus(next: SaveStatus, err?: string) {
    status.value = next
    lastError.value = err ?? null
    if (next === 'saving') savingStartedAt = Date.now()
    if (next === 'saved') {
      savedIndicatorTimer = setTimeout(() => {
        if (status.value === 'saved') status.value = 'idle'
      }, SAVED_INDICATOR_MS)
    }
  }

  function isDirty(property: string): boolean {
    if (property in pending) return true
    if (property in timers) return true
    const last = lastCommitAt[property]
    if (last && Date.now() - last < dirtyWindowMs) return true
    return false
  }

  function isContentDirty(): boolean {
    if (pendingContent !== null) return true
    if (contentTimer !== null) return true
    const last = lastCommitAt['__content__']
    return !!(last && Date.now() - last < dirtyWindowMs)
  }

  function isRelationsDirty(): boolean {
    return relationsDirty || relationsTimer !== null
  }

  function recordServerSnapshot(entity: Entity) {
    for (const k of Object.keys(lastSeenServer)) delete lastSeenServer[k]
    if (entity.properties) {
      for (const [k, v] of Object.entries(entity.properties)) {
        lastSeenServer[k] = v
      }
    }
    lastSeenContent = entity.content ?? ''
  }

  if (opts.initialServerSnapshot) {
    recordServerSnapshot(opts.initialServerSnapshot)
  }

  function scheduleFieldSave(property: string, value: unknown) {
    if (!propertyChannelEnabled) throw new AutoSaveChannelDisabledError('property')
    if (!(property in pending)) pendingCount.value++
    pending[property] = { value, enqueuedAt: Date.now() }
    if (timers[property]) clearTimeout(timers[property])
    timers[property] = setTimeout(() => fireProperty(property), fieldDebounceMs)
  }

  function scheduleUnset(property: string) {
    if (!propertyChannelEnabled) throw new AutoSaveChannelDisabledError('property')
    if (!(property in pending)) pendingCount.value++
    pending[property] = { value: UNSET, enqueuedAt: Date.now() }
    if (timers[property]) clearTimeout(timers[property])
    timers[property] = setTimeout(() => fireProperty(property), fieldDebounceMs)
  }

  function scheduleContentSave(content: string) {
    if (!contentChannelEnabled) throw new AutoSaveChannelDisabledError('content')
    if (pendingContent === null) pendingCount.value++
    pendingContent = { value: content, enqueuedAt: Date.now() }
    if (contentTimer) clearTimeout(contentTimer)
    contentTimer = setTimeout(() => fireContent(), contentDebounceMs)
  }

  function scheduleRelationsChange() {
    if (!relationsChannelEnabled) throw new AutoSaveChannelDisabledError('relations')
    relationsDirty = true
    if (relationsTimer) clearTimeout(relationsTimer)
    relationsTimer = setTimeout(() => fireRelations(), relationsDebounceMs)
  }

  function fireProperty(property: string) {
    const entry = pending[property]
    if (!entry) return
    delete timers[property]
    delete pending[property]
    pendingCount.value = Math.max(0, pendingCount.value - 1)

    // No-op suppression
    if (entry.value !== UNSET && deepEqual(entry.value, lastSeenServer[property])) {
      return
    }

    const enqueuedAt = entry.enqueuedAt
    const isUnset = entry.value === UNSET
    const propertyValue = entry.value

    queueTail = queueTail.then(runPatch, runPatch)

    async function runPatch() {
      const ac = new AbortController()
      currentAbort = ac
      inFlightCount.value++
      setStatus('saving')
      try {
        const patch: EntityPatch = isUnset
          ? { properties_unset: [property] }
          : { properties: { [property]: propertyValue } }
        // Bundle relations if dirty (C2: relations bundling table).
        attachRelations(patch)
        const response = await entitiesStore.update(
          opts.getEntityType(), opts.getEntityId(), patch, undefined, ac.signal,
        )
        mergeServerResponse(response)
        categorizeWarnings(response.warnings)
        if (relationsDirty) {
          relationsDirty = false
          if (relationsTimer) { clearTimeout(relationsTimer); relationsTimer = null }
        }
        lastCommitAt[property] = Date.now()
        if (fieldErrors.value[property]) {
          const next = { ...fieldErrors.value }
          delete next[property]
          fieldErrors.value = next
        }
        setStatus('saved')
      } catch (err: unknown) {
        const message = getErrorMessage(err, 'Save failed')
        const newer = pending[property]
        const isLatestIntent = !newer || newer.enqueuedAt <= enqueuedAt
        if (isLatestIntent) {
          fieldErrors.value = { ...fieldErrors.value, [property]: message }
          setStatus('error', message)
          opts.onError(message, { status: getErrorStatus(err), property, channel: 'property' })
        }
      } finally {
        inFlightCount.value--
        if (currentAbort === ac) currentAbort = null
      }
    }
  }

  function fireContent() {
    if (pendingContent === null) return
    const value = pendingContent.value
    pendingContent = null
    contentTimer = null
    pendingCount.value = Math.max(0, pendingCount.value - 1)

    if (value === lastSeenContent) return

    queueTail = queueTail.then(runPatch, runPatch)

    async function runPatch() {
      const ac = new AbortController()
      currentAbort = ac
      inFlightCount.value++
      setStatus('saving')
      try {
        const patch: EntityPatch = { content: value }
        attachRelations(patch)
        const response = await entitiesStore.update(
          opts.getEntityType(), opts.getEntityId(), patch, undefined, ac.signal,
        )
        mergeServerResponse(response)
        categorizeWarnings(response.warnings)
        if (relationsDirty) {
          relationsDirty = false
          if (relationsTimer) { clearTimeout(relationsTimer); relationsTimer = null }
        }
        lastCommitAt['__content__'] = Date.now()
        contentError.value = null
        setStatus('saved')
      } catch (err: unknown) {
        const message = getErrorMessage(err, 'Save failed')
        if (pendingContent === null) {
          contentError.value = message
          setStatus('error', message)
          opts.onError(message, { status: getErrorStatus(err), channel: 'content' })
        }
      } finally {
        inFlightCount.value--
        if (currentAbort === ac) currentAbort = null
      }
    }
  }

  function fireRelations() {
    if (!relationsDirty) return
    if (relationsTimer) { clearTimeout(relationsTimer); relationsTimer = null }
    const body = opts.buildRelationsBody()
    if (!body || Object.keys(body).length === 0) {
      // Pristine — nothing to send. Clear the dirty bit; the form may
      // have rolled back its own state.
      relationsDirty = false
      return
    }

    queueTail = queueTail.then(runPatch, runPatch)

    async function runPatch() {
      const ac = new AbortController()
      currentAbort = ac
      inFlightCount.value++
      setStatus('saving')
      try {
        const patch: EntityPatch = { relations: body as unknown as EntityPatch['relations'] }
        const response = await entitiesStore.update(
          opts.getEntityType(), opts.getEntityId(), patch, undefined, ac.signal,
        )
        mergeServerResponse(response)
        categorizeWarnings(response.warnings)
        relationsDirty = false
        lastCommitAt['__relations__'] = Date.now()
        setStatus('saved')
      } catch (err: unknown) {
        const message = getErrorMessage(err, 'Save failed')
        setStatus('error', message)
        opts.onError(message, { status: getErrorStatus(err), channel: 'relations' })
      } finally {
        inFlightCount.value--
        if (currentAbort === ac) currentAbort = null
      }
    }
  }

  // attachRelations is called from fireProperty/fireContent to bundle
  // the relations body when relationsDirty is set. Mutates `patch` in
  // place. Cleanup of `relationsDirty` happens in the runPatch caller
  // after the response is processed.
  function attachRelations(patch: EntityPatch) {
    if (!relationsDirty) return
    const body = opts.buildRelationsBody()
    if (!body || Object.keys(body).length === 0) {
      // Pristine — drop the dirty flag without emitting a key.
      relationsDirty = false
      if (relationsTimer) { clearTimeout(relationsTimer); relationsTimer = null }
      return
    }
    patch.relations = body as unknown as EntityPatch['relations']
    if (relationsTimer) { clearTimeout(relationsTimer); relationsTimer = null }
  }

  // categorizeWarnings consumes the server response's warnings and
  // routes each to the appropriate UI surface.
  function categorizeWarnings(warnings: AutoSaveWarning[] | undefined) {
    if (!warnings || warnings.length === 0) return
    for (const w of warnings) {
      const path = w.path ?? ''
      const propMatch = path.match(/^\/properties\/([^/]+)/)
      if (propMatch) {
        fieldWarnings.value = { ...fieldWarnings.value, [propMatch[1]]: w }
        continue
      }
      const unsetMatch = path.match(/^\/properties_unset\/(\d+)/)
      if (unsetMatch) {
        // Index-keyed; no field name on the path. Surface against
        // unsetWarnings indexed by position via a fallback key.
        fieldWarnings.value = { ...fieldWarnings.value, [`__unset_${unsetMatch[1]}`]: w }
        continue
      }
      if (path === '/content' || path.startsWith('/content/')) {
        contentWarning.value = w
        continue
      }
      const relMatch = path.match(/^\/relations\/([^/]+)/)
      if (relMatch) {
        const bodyKey = relMatch[1]
        const direction = w.direction === 'incoming' ? 'incoming' : 'outgoing'
        const canonical = direction === 'incoming'
          ? opts.inverseToCanonical.get(bodyKey) ?? bodyKey
          : bodyKey
        const widgetId = `${canonical}-${direction}` as WidgetId
        relationWarnings.value = { ...relationWarnings.value, [widgetId]: w }
        continue
      }
      // Unrecognized — leave for console; no UI surface.
    }
  }

  function mergeServerResponse(entity: Entity) {
    // Defence in depth: a disabled channel must not have any pending
    // state. If it does, schedule* slipped past the throw guard or a
    // previous call mutated this instance directly. Either way, fail
    // loud — the disabled-channel invariant is load-bearing for the
    // EntityDetail content-only instance.
    if (!propertyChannelEnabled && (Object.keys(pending).length > 0 || Object.keys(timers).length > 0)) {
      throw new Error('useAutoSave: property channel disabled but pending state observed')
    }
    if (!contentChannelEnabled && (pendingContent !== null || contentTimer !== null)) {
      throw new Error('useAutoSave: content channel disabled but pending state observed')
    }
    if (!relationsChannelEnabled && (relationsDirty || relationsTimer !== null)) {
      throw new Error('useAutoSave: relations channel disabled but pending state observed')
    }

    if (entity.properties) {
      for (const [k, v] of Object.entries(entity.properties)) {
        // S5: always update lastSeenServer from server, regardless of dirty.
        // Done even when the property channel is disabled so the
        // baseline stays valid for any later re-init.
        lastSeenServer[k] = v
        // Only mutate formData for non-dirty fields. Skip entirely
        // when the property channel is disabled — the caller doesn't
        // own a writable formData ref for properties in that case.
        if (!propertyChannelEnabled) continue
        if (k in pending) continue
        if (k in timers) continue
        opts.applyServerProperty(k, v)
      }
      // Properties that disappeared from the server response (server-
      // side unset by automation): clear them locally too, but only
      // when the field isn't dirty and the channel is enabled.
      for (const k of Object.keys(lastSeenServer)) {
        if (!(k in entity.properties) && !(k in pending) && !(k in timers)) {
          if (propertyChannelEnabled) opts.applyServerProperty(k, undefined)
          delete lastSeenServer[k]
        }
      }
    }
    if (entity.content !== undefined && pendingContent === null && contentTimer === null) {
      // Baseline always updates; apply callback skipped when the
      // content channel is disabled.
      lastSeenContent = entity.content
      if (contentChannelEnabled) opts.applyServerContent(entity.content)
    }
  }

  function revertField(property: string) {
    if (timers[property]) {
      clearTimeout(timers[property])
      delete timers[property]
    }
    if (property in pending) {
      delete pending[property]
      pendingCount.value = Math.max(0, pendingCount.value - 1)
    }
    if (property in lastSeenServer) {
      opts.applyServerProperty(property, lastSeenServer[property])
    } else {
      opts.applyServerProperty(property, undefined)
    }
    if (fieldErrors.value[property]) {
      const next = { ...fieldErrors.value }
      delete next[property]
      fieldErrors.value = next
    }
  }

  function revertContent() {
    if (contentTimer) {
      clearTimeout(contentTimer)
      contentTimer = null
    }
    if (pendingContent !== null) {
      pendingContent = null
      pendingCount.value = Math.max(0, pendingCount.value - 1)
    }
    opts.applyServerContent(lastSeenContent)
    contentError.value = null
  }

  // C4: typed CommitResult, timeout owner is the composable, aborts
  // in-flight saves on timeout.
  function commitImmediately(timeoutMs = 10_000): Promise<CommitResult> {
    // Flush per-property timers, content timer, relations timer.
    for (const p of Object.keys(timers)) {
      const t = timers[p]
      if (t) clearTimeout(t)
      fireProperty(p)
    }
    if (contentTimer) {
      clearTimeout(contentTimer)
      contentTimer = null
      fireContent()
    }
    if (relationsTimer || relationsDirty) {
      if (relationsTimer) { clearTimeout(relationsTimer); relationsTimer = null }
      fireRelations()
    }
    return new Promise<CommitResult>((resolve) => {
      const timer = setTimeout(() => {
        // Abort whatever is currently in flight; leave the rest of the
        // chain to die naturally with an aborted error.
        if (currentAbort) {
          currentAbort.abort()
        }
        resolve({ settled: false, error: 'timeout' })
      }, timeoutMs)
      queueTail
        .then(() => resolve({ settled: true }))
        .catch((err: unknown) => {
          resolve({ settled: true, error: getErrorMessage(err, 'Save failed') })
        })
        .finally(() => clearTimeout(timer))
    })
  }

  return {
    status: computed(() => status.value),
    lastError: computed(() => lastError.value),
    inFlightCount: computed(() => inFlightCount.value),
    pendingCount: computed(() => pendingCount.value),
    fieldErrors: computed(() => fieldErrors.value),
    fieldWarnings: computed(() => fieldWarnings.value),
    contentError: computed(() => contentError.value),
    contentWarning: computed(() => contentWarning.value),
    relationWarnings: computed(() => relationWarnings.value),
    isDirty,
    isContentDirty,
    isRelationsDirty,
    scheduleFieldSave,
    scheduleUnset,
    scheduleContentSave,
    scheduleRelationsChange,
    commitImmediately,
    revertField,
    revertContent,
    recordServerSnapshot,
    mergeServerResponse,
  }
}

// Extract the HTTP status from a thrown error, when available. Returns
// undefined for non-ApiError rejections (network errors, cancellations,
// programming bugs) — callers should treat undefined as "unknown status,
// not necessarily success." Used to populate AutoSaveErrorInfo.status
// for the host's 401/403 dispatch.
function getErrorStatus(err: unknown): number | undefined {
  return err instanceof ApiError ? err.status : undefined
}

function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true
  if (a == null || b == null) return a === b
  if (typeof a !== 'object' || typeof b !== 'object') return false
  if (Array.isArray(a) !== Array.isArray(b)) return false
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false
    for (let i = 0; i < a.length; i++) if (!deepEqual(a[i], b[i])) return false
    return true
  }
  const ao = a as Record<string, unknown>
  const bo = b as Record<string, unknown>
  const ak = Object.keys(ao)
  const bk = Object.keys(bo)
  if (ak.length !== bk.length) return false
  for (const k of ak) if (!deepEqual(ao[k], bo[k])) return false
  return true
}
