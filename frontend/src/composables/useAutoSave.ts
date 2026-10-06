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
// * No `If-Match` on PATCH. The entity ETag covers every field, so it
//   would refuse a save after ANY other edit. Each PATCH instead carries
//   per-field preconditions (TKT-2VDVHF): the version token of each field
//   it writes, as last seen from the server. On a 412 the composable merges
//   the conflicting fields three-way and retries; see `sendPatch`.
// * `lastSeenServer` is only updated from server responses
//   (via `mergeServerResponse`). The WIP wrote client-sent values
//   directly, which masked server-side automation drift.

import { ref, computed, type Ref } from 'vue'
import type { Entity, FieldConflicts, ModernRelationsField, Preconditions } from '@/types'
import { getEntity, type EntityPatch } from '@/api/entities'
import { ApiError, getErrorMessage } from '@/api/errors'
import { useEntitiesStore } from '@/stores/entities'
import { mergeProperty, mergeRelations, mergeText } from './autoSaveMerge'

// Sentinel for "unset this property" pending entries. Distinct from
// undefined so we can tell apart "delete the key" from "set to
// undefined" (which the API treats the same as null/"").
const UNSET = Symbol('unset')

const SAVED_INDICATOR_MS = 1200
// Minimum time the 'saving' state stays visible. Even when a PATCH
// resolves in 50ms, the indicator holds 'saving' for this long so the
// user perceives a smooth idle → saving → saved transition.
const MIN_SAVING_VISIBLE_MS = 600
// PATCH attempts per save when preconditions fail. Each failed attempt
// either merges (someone else changed a field this save writes) or resends
// unchanged (someone else changed another field between the check and the
// write). Three is enough for a few concurrent editors; beyond it the save
// fails like any other error and the next edit tries again.
const MAX_CONFLICT_ATTEMPTS = 3

export const CONFLICT_MESSAGE =
  'Someone else changed this field while you were editing. Your change was not saved. ' +
  'Edit it again to overwrite their value.'
export const CONTENT_CONFLICT_MESSAGE =
  'Someone else changed the same lines of the body. Your change was not saved. ' +
  'Edit it again to overwrite their version.'
export const RELATIONS_CONFLICT_MESSAGE =
  'Someone else changed these relations and the changes could not be combined. ' +
  'Your relation change was not saved.'

// A merge base: the server value the local state derives from, and the
// version token the server issued for it. `token` is undefined when the
// server sent none, which disables the precondition for that field.
interface Base<T> {
  value: T
  token: string | undefined
}

// What one PATCH round wrote and what the conflict handling did to it.
interface SendResult {
  response: Entity
  written: { props: Set<string>; content: boolean; relations: boolean }
  // The relations body written, after any merge; undefined when the save
  // wrote none, including when the conflict handling dropped it.
  sentRelations: ModernRelationsField | undefined
  // The content or relations written differ from the local edit, because
  // they were merged with someone else's change.
  mergedContent: boolean
  mergedRelations: boolean
  conflicts: Conflicts
}

// Fields a save could not write, with the bases they move to once the
// save's response is merged. Bases move there and not while resolving, so
// a save that finally fails leaves every base where the form still is.
interface Conflicts {
  props: string[]
  content: boolean
  relations: boolean
  // Per conflicting property: the other side's value and token.
  propBases: Record<string, Base<unknown>>
  // For a conflicting body: the other side's body and token, and the text
  // the editor shows (their clean hunks, our side of each conflict).
  contentBase?: Base<string>
  contentShown?: string
}

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
  // Called when the PATCH is about to be sent, never earlier: the body is a
  // delta against the edges saved so far, so it must see every earlier save.
  buildRelationsBody: () => ModernRelationsField | null
  // Called with a relations body the server accepted, so the form can
  // advance the edges its next delta is computed against.
  onRelationsSaved?: (body: ModernRelationsField) => void
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

  // Merge bases (TKT-2VDVHF). Unlike lastSeenServer, a base moves only when
  // the local state moves with it: when the form takes the server value, or
  // when a save of that field succeeds. A base that ran ahead of the form
  // would let the next save overwrite a change the user never saw.
  const propBase: Record<string, Base<unknown>> = Object.create(null)
  let contentBase: Base<string> = { value: '', token: undefined }
  let relationsBase: Base<Record<string, string[]>> = { value: {}, token: undefined }

  const pending: Record<string, PendingEntry> = Object.create(null)
  let pendingContent: { value: string; enqueuedAt: number } | null = null
  const timers: Record<string, ReturnType<typeof setTimeout>> = Object.create(null)
  let contentTimer: ReturnType<typeof setTimeout> | null = null

  // Relations channel: a single boolean (not per-relation). The form
  // owns the Map; the composable just remembers "kick the queue on
  // next debounce fire."
  let relationsDirty = false
  // Relations bodies sent and not yet answered. A body takes the dirty bit
  // when it is built, so an edit made while it is in flight sets the bit
  // again and is sent next, and a failed body hands the bit back.
  let relationsSending = 0
  let relationsTimer: ReturnType<typeof setTimeout> | null = null

  // Writes already taken off `pending` / `pendingContent` that are queued
  // behind another save or in flight. They are still unsaved local state,
  // so a response to an EARLIER save, or a fresh snapshot, must neither
  // apply over them nor move their base.
  const queuedProps: Record<string, number> = Object.create(null)
  let queuedContent = 0

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
    return relationsDirty || relationsTimer !== null || relationsSending > 0
  }

  // holds* report unsaved local state: scheduled, debouncing, or queued.
  function holdsProp(property: string): boolean {
    return property in pending || property in timers || (queuedProps[property] ?? 0) > 0
  }

  function holdsContent(): boolean {
    return pendingContent !== null || contentTimer !== null || queuedContent > 0
  }

  function recordServerSnapshot(entity: Entity) {
    for (const k of Object.keys(lastSeenServer)) delete lastSeenServer[k]
    if (entity.properties) {
      for (const [k, v] of Object.entries(entity.properties)) {
        lastSeenServer[k] = v
      }
    }
    lastSeenContent = entity.content ?? ''

    // A field with unsaved local state keeps its base: that state derives
    // from the old one, not from whatever this snapshot carries.
    const versions = entity._versions
    for (const k of Object.keys(propBase)) if (!holdsProp(k)) delete propBase[k]
    for (const [k, token] of Object.entries(versions?.properties ?? {})) {
      if (!holdsProp(k)) propBase[k] = { value: entity.properties?.[k], token }
    }
    if (!holdsContent()) contentBase = { value: entity.content ?? '', token: versions?.content }
    if (!isRelationsDirty()) {
      relationsBase = { value: { ...(entity.relations ?? {}) }, token: versions?.relations }
    }
  }

  if (opts.initialServerSnapshot) {
    recordServerSnapshot(opts.initialServerSnapshot)
  }

  function scheduleFieldSave(property: string, value: unknown) {
    if (!propertyChannelEnabled) throw new AutoSaveChannelDisabledError('property')
    if (!(property in pending)) pendingCount.value++
    pending[property] = { value, enqueuedAt: Date.now() }
    if (timers[property]) clearTimeout(timers[property])
    timers[property] = setTimeout(() => fireDue(property), fieldDebounceMs)
  }

  function scheduleUnset(property: string) {
    if (!propertyChannelEnabled) throw new AutoSaveChannelDisabledError('property')
    if (!(property in pending)) pendingCount.value++
    pending[property] = { value: UNSET, enqueuedAt: Date.now() }
    if (timers[property]) clearTimeout(timers[property])
    timers[property] = setTimeout(() => fireDue(property), fieldDebounceMs)
  }

  // `debounceMs` overrides the channel's debounce for this call. One instance
  // can then serve both a discrete edit that should land at once (a checkbox
  // toggle) and typing, where a save per pause is enough.
  function scheduleContentSave(content: string, debounceMs = contentDebounceMs) {
    if (!contentChannelEnabled) throw new AutoSaveChannelDisabledError('content')
    if (pendingContent === null) pendingCount.value++
    pendingContent = { value: content, enqueuedAt: Date.now() }
    if (contentTimer) clearTimeout(contentTimer)
    contentTimer = setTimeout(() => fireContent(), debounceMs)
  }

  function scheduleRelationsChange() {
    if (!relationsChannelEnabled) throw new AutoSaveChannelDisabledError('relations')
    relationsDirty = true
    if (relationsTimer) clearTimeout(relationsTimer)
    relationsTimer = setTimeout(() => fireRelations(), relationsDebounceMs)
  }

  /**
   * Fire `property` together with every other property whose debounce has
   * already elapsed, as ONE patch (TKT-7S5735 AC4).
   *
   * Merging is not an optimization here, it is a correctness property. An
   * accepted `clear_when_hidden` decision is a set of changes the user approved
   * together — the trigger's new value plus the unset of what it hid. Emitting
   * them as separate requests leaves a window in which the entity holds a state
   * the user never approved (trigger changed, dependent field still populated),
   * and if the second request fails that state is what persists.
   *
   * Scope note: this makes an approved DECISION atomic. It does not freeze the
   * form — an unrelated field edited while a confirm dialog is open still
   * debounces and saves on its own, because the gated proposal is not in
   * `pending` yet (that is the whole design). That write is independent of the
   * decision, so it is not part of the set being made atomic.
   *
   * Per-property semantics are preserved inside the batch:
   * - **no-op suppression** is evaluated per entry while building, and a batch
   *   in which every entry is suppressed sends nothing at all (rather than an
   *   empty `{properties:{}}` PATCH, which would be a new write where the
   *   unbatched code made none);
   * - **set/unset of the same property** cannot both appear, because `pending`
   *   is keyed by property and holds one entry — last write wins;
   * - **error attribution** fans out to every property in the batch. This is a
   *   real widening: one 422 now marks N fields. It is the accepted cost of
   *   atomicity, and the alternative (parsing the server's per-field paths back
   *   onto the batch) is what `categorizeWarnings` already does for warnings.
   */
  function fireDue(property: string, flushAll = false) {
    if (!pending[property]) return

    // Collect this property plus every other one that is DUE.
    //
    // Due means "its debounce window has elapsed", not "its timer has already
    // run". Two properties scheduled in the same tick both still hold live
    // timers when the first one fires, so keying off `timers[key]` would never
    // merge them — which is the common case this exists for (an accepted
    // clear_when_hidden decision schedules the trigger and the unset together).
    //
    // A property still absorbing keystrokes has a later deadline and is left
    // alone; pulling it in early would defeat the debounce it is waiting on.
    // `flushAll` overrides that for commitImmediately, where the user is
    // leaving and every pending edit must go out — as ONE patch, so navigating
    // away cannot half-apply a set of changes either.
    const now = Date.now()
    const batch: Array<{ property: string; entry: PendingEntry }> = []
    for (const key of Object.keys(pending)) {
      const entry = pending[key]
      if (!flushAll && key !== property && entry.enqueuedAt + fieldDebounceMs > now) continue
      batch.push({ property: key, entry })
    }

    for (const { property: key } of batch) {
      if (timers[key]) {
        clearTimeout(timers[key])
        delete timers[key]
      }
      delete pending[key]
      pendingCount.value = Math.max(0, pendingCount.value - 1)
    }

    // No-op suppression, per entry.
    const live = batch.filter(
      ({ property: key, entry }) =>
        entry.value === UNSET || !deepEqual(entry.value, lastSeenServer[key])
    )
    if (!live.length) return // every entry suppressed → no request at all

    const properties = live.filter(({ entry }) => entry.value !== UNSET)
    const unsets = live.filter(({ entry }) => entry.value === UNSET).map((e) => e.property)
    const enqueuedAtOf = new Map(live.map(({ property: key, entry }) => [key, entry.enqueuedAt]))
    const keys = live.map((e) => e.property)
    for (const key of keys) queuedProps[key] = (queuedProps[key] ?? 0) + 1
    let released = false
    const release = () => {
      if (released) return
      released = true
      for (const key of keys) if (--queuedProps[key] <= 0) delete queuedProps[key]
    }

    queueTail = queueTail.then(runPatch, runPatch)

    async function runPatch() {
      const ac = new AbortController()
      currentAbort = ac
      inFlightCount.value++
      setStatus('saving')
      let sentRelations: ModernRelationsField | null = null
      try {
        const patch: EntityPatch = {}
        if (properties.length) {
          patch.properties = Object.fromEntries(
            properties.map(({ property: key, entry }) => [key, entry.value])
          )
        }
        if (unsets.length) patch.properties_unset = unsets
        // Bundle relations if dirty (C2: relations bundling table).
        sentRelations = attachRelations(patch)
        const result = await sendPatch(patch, ac.signal)
        release()
        // A relations body the conflict handling dropped was not written:
        // it stays held, and the finally block hands the bit back.
        if (sentRelations && result.sentRelations) {
          opts.onRelationsSaved?.(result.sentRelations)
          relationsSending--
          sentRelations = null
        }
        const response = result.response
        mergeServerResponse(response, result)
        categorizeWarnings(response.warnings)
        const now = Date.now()
        let nextErrors: Record<string, string> | null = null
        for (const key of keys) {
          lastCommitAt[key] = now
          if (fieldErrors.value[key]) {
            nextErrors ??= { ...fieldErrors.value }
            delete nextErrors[key]
          }
        }
        if (nextErrors) fieldErrors.value = nextErrors
        if (!reportConflicts(result, 'property')) setStatus('saved')
      } catch (err: unknown) {
        const message = getErrorMessage(err, 'Save failed')
        // Attribute to every property in the batch whose intent is still the
        // latest — a field re-edited while this request was in flight has a
        // newer intent and must not be marked for this failure.
        let nextErrors: Record<string, string> | null = null
        let attributed: string | undefined
        for (const key of keys) {
          const newer = pending[key]
          if (newer && newer.enqueuedAt > (enqueuedAtOf.get(key) ?? 0)) continue
          nextErrors ??= { ...fieldErrors.value }
          nextErrors[key] = message
          attributed ??= key
        }
        if (nextErrors) {
          fieldErrors.value = nextErrors
          setStatus('error', message)
          opts.onError(message, {
            status: getErrorStatus(err),
            property: attributed,
            channel: 'property',
          })
        }
      } finally {
        release()
        // A body still held here was not accepted: hand the bit back.
        if (sentRelations) {
          relationsSending--
          relationsDirty = true
        }
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
    queuedContent++
    let released = false
    const release = () => {
      if (!released) queuedContent--
      released = true
    }

    queueTail = queueTail.then(runPatch, runPatch)

    async function runPatch() {
      const ac = new AbortController()
      currentAbort = ac
      inFlightCount.value++
      setStatus('saving')
      let sentRelations: ModernRelationsField | null = null
      try {
        const patch: EntityPatch = { content: value }
        sentRelations = attachRelations(patch)
        const result = await sendPatch(patch, ac.signal)
        release()
        // A relations body the conflict handling dropped was not written:
        // it stays held, and the finally block hands the bit back.
        if (sentRelations && result.sentRelations) {
          opts.onRelationsSaved?.(result.sentRelations)
          relationsSending--
          sentRelations = null
        }
        const response = result.response
        mergeServerResponse(response, result)
        categorizeWarnings(response.warnings)
        lastCommitAt['__content__'] = Date.now()
        contentError.value = null
        if (!reportConflicts(result, 'content')) setStatus('saved')
      } catch (err: unknown) {
        const message = getErrorMessage(err, 'Save failed')
        if (pendingContent === null) {
          contentError.value = message
          setStatus('error', message)
          opts.onError(message, { status: getErrorStatus(err), channel: 'content' })
        }
      } finally {
        release()
        // A body still held here was not accepted: hand the bit back.
        if (sentRelations) {
          relationsSending--
          relationsDirty = true
        }
        inFlightCount.value--
        if (currentAbort === ac) currentAbort = null
      }
    }
  }

  function fireRelations() {
    if (!relationsDirty) return
    if (relationsTimer) {
      clearTimeout(relationsTimer)
      relationsTimer = null
    }
    queueTail = queueTail.then(runPatch, runPatch)

    async function runPatch() {
      // Built here, after every earlier queued save has landed: the body is
      // a delta against what those saves confirmed.
      if (!relationsDirty) return
      const body = opts.buildRelationsBody()
      if (!body || Object.keys(body).length === 0) {
        // Pristine — nothing to send. Clear the dirty bit; the form may
        // have rolled back its own state.
        relationsDirty = false
        return
      }
      relationsDirty = false
      relationsSending++
      let held = true
      const ac = new AbortController()
      currentAbort = ac
      inFlightCount.value++
      setStatus('saving')
      try {
        const patch: EntityPatch = { relations: body }
        const result = await sendPatch(patch, ac.signal)
        if (result.sentRelations) {
          opts.onRelationsSaved?.(result.sentRelations)
          relationsSending--
          held = false
        }
        const response = result.response
        mergeServerResponse(response, result)
        categorizeWarnings(response.warnings)
        lastCommitAt['__relations__'] = Date.now()
        if (!reportConflicts(result, 'relations')) setStatus('saved')
      } catch (err: unknown) {
        const message = getErrorMessage(err, 'Save failed')
        setStatus('error', message)
        opts.onError(message, { status: getErrorStatus(err), channel: 'relations' })
      } finally {
        if (held) {
          relationsSending--
          relationsDirty = true
        }
        inFlightCount.value--
        if (currentAbort === ac) currentAbort = null
      }
    }
  }

  // preconditionsFor names, for each field the patch writes, the token of
  // its merge base. A field without a token (older server, or a snapshot
  // taken from a response that carries none) is sent unchecked.
  function preconditionsFor(patch: EntityPatch): Preconditions | undefined {
    const pre: Preconditions = {}
    const keys = [...Object.keys(patch.properties ?? {}), ...(patch.properties_unset ?? [])]
    for (const k of keys) {
      const token = propBase[k]?.token
      if (token !== undefined) (pre.properties ??= {})[k] = token
    }
    if (patch.content !== undefined && contentBase.token !== undefined)
      pre.content = contentBase.token
    if (patch.relations && relationsBase.token !== undefined) pre.relations = relationsBase.token
    return Object.keys(pre).length ? pre : undefined
  }

  /**
   * Send one save, resolving precondition failures (TKT-2VDVHF).
   *
   * A 412 with an EMPTY `conflicts` means another write landed between the
   * server's check and its write without touching the fields this save
   * names; the same request is resent. A 412 naming fields means someone
   * else changed them: the current entity is fetched and each named field
   * is merged against its base (see autoSaveMerge.ts). Fields that merge
   * cleanly are resent with fresh tokens; fields that cannot are dropped
   * from the request and reported, and nothing is written for them.
   *
   * The server never retries on the client's behalf, because only the
   * client holds the base needed to merge.
   */
  async function sendPatch(initial: EntityPatch, signal: AbortSignal): Promise<SendResult> {
    let patch: EntityPatch = { ...initial }
    let pre = preconditionsFor(patch)
    const conflicts: Conflicts = { props: [], content: false, relations: false, propBases: {} }
    let mergedContent = false
    let mergedRelations = false
    let fresh: Entity | null = null
    for (let attempt = 1; ; attempt++) {
      if (fresh && isEmptyPatch(patch)) {
        // Every field resolved to "nothing to write". Answer with the state
        // just fetched so the form still takes the other side's values.
        return {
          response: fresh,
          written: writtenBy({}),
          sentRelations: undefined,
          mergedContent,
          mergedRelations,
          conflicts,
        }
      }
      try {
        const body = pre ? { ...patch, preconditions: pre } : patch
        const response = await entitiesStore.update(
          opts.getEntityType(),
          opts.getEntityId(),
          body,
          undefined,
          signal
        )
        return {
          response,
          written: writtenBy(patch),
          sentRelations: patch.relations,
          mergedContent,
          mergedRelations,
          conflicts,
        }
      } catch (err: unknown) {
        const failed = preconditionConflicts(err)
        if (!failed || !pre || attempt >= MAX_CONFLICT_ATTEMPTS) throw err
        await backoff(attempt, signal)
        if (isEmptyConflicts(failed)) continue
        fresh = await fetchCurrent(patch, signal)
        const next = resolveConflicts(patch, pre, fresh, failed, conflicts)
        patch = next.patch
        pre = next.pre
        mergedContent ||= next.mergedContent
        mergedRelations ||= next.mergedRelations
      }
    }
  }

  // fetchCurrent reads the entity as it is now, bypassing the store cache.
  // It includes the outgoing relation targets the patch writes, because a
  // merged relations entry needs each target's type and the relations map
  // carries ids only.
  //
  // No `?world=`: the server then reads in its default world, which is the
  // view a PATCH computes its tokens in. Naming `default` instead is refused
  // with a 400 on a schema that declares worlds.
  async function fetchCurrent(patch: EntityPatch, signal: AbortSignal): Promise<Entity> {
    const outgoing = Object.keys(patch.relations ?? {}).filter(
      (k) => !opts.inverseToCanonical.has(k)
    )
    return getEntity(
      opts.getEntityType(),
      opts.getEntityId(),
      outgoing.length ? { include: outgoing.join(',') } : {},
      signal
    )
  }

  function resolveConflicts(
    patch: EntityPatch,
    pre: Preconditions,
    fresh: Entity,
    failed: FieldConflicts,
    conflicts: Conflicts
  ): {
    patch: EntityPatch
    pre: Preconditions | undefined
    mergedContent: boolean
    mergedRelations: boolean
  } {
    const next: EntityPatch = { ...patch }
    if (patch.properties) next.properties = { ...patch.properties }
    if (patch.properties_unset) next.properties_unset = [...patch.properties_unset]
    const preProps: Record<string, string> = { ...(pre.properties ?? {}) }
    const nextPre: Preconditions = { ...pre }
    const tokens = fresh._versions
    let mergedContent = false
    let mergedRelations = false

    for (const k of Object.keys(failed.properties ?? {})) {
      const ours = next.properties && k in next.properties ? next.properties[k] : undefined
      const theirs = fresh.properties?.[k]
      const token = tokens?.properties[k]
      const decision = mergeProperty(propBase[k]?.value, ours, theirs, deepEqual)
      if (decision.kind === 'write' && token !== undefined) {
        preProps[k] = token
        continue
      }
      if (next.properties) delete next.properties[k]
      if (next.properties_unset)
        next.properties_unset = next.properties_unset.filter((u) => u !== k)
      delete preProps[k]
      // 'same' needs no base here: the response carries their value, which
      // the form takes like any other server value.
      if (decision.kind === 'conflict') {
        conflicts.props.push(k)
        conflicts.propBases[k] = { value: theirs, token }
      }
    }
    if (next.properties && !Object.keys(next.properties).length) delete next.properties
    if (next.properties_unset && !next.properties_unset.length) delete next.properties_unset

    if (failed.content && next.content !== undefined) {
      const theirs = fresh.content ?? ''
      const m = mergeText(contentBase.value, next.content, theirs)
      delete next.content
      delete nextPre.content
      if (m.ok && m.merged === theirs) {
        // Their body already holds our edit: nothing to write.
        mergedContent = true
      } else if (m.ok && tokens) {
        mergedContent = m.merged !== patch.content
        next.content = m.merged
        nextPre.content = tokens.content
      } else {
        // Unmergeable, or merged without a token to guard the resend.
        conflicts.content = true
        conflicts.contentBase = { value: theirs, token: tokens?.content }
        conflicts.contentShown = m.ok ? patch.content : m.oursInConflicts
      }
    }

    if (failed.relations && next.relations) {
      const merged = tokens
        ? mergeRelations(
            relationsBase.value,
            next.relations,
            fresh.relations ?? {},
            (key) => opts.inverseToCanonical.has(key),
            (id) => fresh.included?.[id]?.type
          )
        : null
      if (merged && tokens) {
        next.relations = merged
        nextPre.relations = tokens.relations
        mergedRelations = true
      } else {
        delete next.relations
        delete nextPre.relations
        conflicts.relations = true
      }
    }

    if (Object.keys(preProps).length) nextPre.properties = preProps
    else delete nextPre.properties
    return {
      patch: next,
      pre: Object.keys(nextPre).length ? nextPre : undefined,
      mergedContent,
      mergedRelations,
    }
  }

  // reportConflicts surfaces fields a save could not write. Returns true when
  // there were any, so the caller shows the error state instead of 'saved'.
  function reportConflicts(result: SendResult, channel: AutoSaveErrorInfo['channel']): boolean {
    const { props, content, relations } = result.conflicts
    if (!props.length && !content && !relations) return false
    if (props.length) {
      const next = { ...fieldErrors.value }
      for (const k of props) next[k] = CONFLICT_MESSAGE
      fieldErrors.value = next
    }
    if (content) contentError.value = CONTENT_CONFLICT_MESSAGE
    const message = content
      ? CONTENT_CONFLICT_MESSAGE
      : props.length
        ? CONFLICT_MESSAGE
        : RELATIONS_CONFLICT_MESSAGE
    setStatus('error', message)
    opts.onError(message, {
      status: 412,
      property: props[0],
      channel: content ? 'content' : props.length ? 'property' : relations ? 'relations' : channel,
    })
    return true
  }

  // attachRelations is called from fireDue/fireContent to bundle
  // the relations body when relationsDirty is set. Mutates `patch` in
  // place and takes the dirty bit (see relationsSending); the runPatch
  // caller confirms the body or hands the bit back. Returns the attached body, or null.
  function attachRelations(patch: EntityPatch): ModernRelationsField | null {
    if (!relationsDirty) return null
    const body = opts.buildRelationsBody()
    if (!body || Object.keys(body).length === 0) {
      // Pristine — drop the dirty flag without emitting a key.
      relationsDirty = false
      if (relationsTimer) {
        clearTimeout(relationsTimer)
        relationsTimer = null
      }
      return null
    }
    patch.relations = body
    relationsDirty = false
    relationsSending++
    if (relationsTimer) {
      clearTimeout(relationsTimer)
      relationsTimer = null
    }
    return body
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
        const canonical =
          direction === 'incoming' ? (opts.inverseToCanonical.get(bodyKey) ?? bodyKey) : bodyKey
        const widgetId = `${canonical}-${direction}` as WidgetId
        relationWarnings.value = { ...relationWarnings.value, [widgetId]: w }
        continue
      }
      // Unrecognized — leave for console; no UI surface.
    }
  }

  // `sent` describes the save this response answers, when there was one; it
  // decides which merge bases may move (see propBase).
  function mergeServerResponse(entity: Entity, sent?: SendResult) {
    // Defence in depth: a disabled channel must not have any pending
    // state. If it does, schedule* slipped past the throw guard or a
    // previous call mutated this instance directly. Either way, fail
    // loud — the disabled-channel invariant is load-bearing for the
    // EntityDetail content-only instance.
    if (
      !propertyChannelEnabled &&
      (Object.keys(pending).length > 0 || Object.keys(timers).length > 0)
    ) {
      throw new Error('useAutoSave: property channel disabled but pending state observed')
    }
    if (!contentChannelEnabled && (pendingContent !== null || contentTimer !== null)) {
      throw new Error('useAutoSave: content channel disabled but pending state observed')
    }
    if (!relationsChannelEnabled && (relationsDirty || relationsTimer !== null)) {
      throw new Error('useAutoSave: relations channel disabled but pending state observed')
    }

    const versions = entity._versions
    const conflicted = new Set(sent?.conflicts.props ?? [])
    // A field's base follows the form: it moves when the form takes the
    // server value, or when this response answers a save of that field.
    const moveBase = (k: string, value: unknown, applied: boolean) => {
      if (conflicted.has(k)) {
        // "Edit it again to overwrite": the next save of k is checked
        // against their value, which the user has now been told about.
        const theirs = sent?.conflicts.propBases[k]
        if (theirs) propBase[k] = theirs
        return
      }
      if (!versions) return
      if (applied || sent?.written.props.has(k)) {
        propBase[k] = { value, token: versions.properties[k] }
      }
    }

    if (entity.properties) {
      for (const [k, v] of Object.entries(entity.properties)) {
        // S5: always update lastSeenServer from server, regardless of dirty.
        // Done even when the property channel is disabled so the
        // baseline stays valid for any later re-init.
        lastSeenServer[k] = v
        // Only mutate formData for non-dirty fields. Skip entirely
        // when the property channel is disabled — the caller doesn't
        // own a writable formData ref for properties in that case. A
        // conflicting field keeps the user's value on screen.
        const apply = propertyChannelEnabled && !holdsProp(k) && !conflicted.has(k)
        if (apply) opts.applyServerProperty(k, v)
        moveBase(k, v, apply)
      }
      // Properties that disappeared from the server response (server-
      // side unset by automation): clear them locally too, but only
      // when the field isn't dirty and the channel is enabled.
      for (const k of Object.keys(lastSeenServer)) {
        if (!(k in entity.properties) && !holdsProp(k)) {
          const apply = propertyChannelEnabled && !conflicted.has(k)
          if (apply) opts.applyServerProperty(k, undefined)
          delete lastSeenServer[k]
          moveBase(k, undefined, apply)
        }
      }
      // Declared properties that are unset on both sides carry a token too.
      for (const k of Object.keys(versions?.properties ?? {})) {
        if (k in entity.properties || k in propBase) continue
        moveBase(k, undefined, !holdsProp(k))
      }
    }
    if (entity.content !== undefined) {
      if (!holdsContent()) {
        // Baseline always updates; apply callback skipped when the
        // content channel is disabled.
        lastSeenContent = entity.content
        const conflict = sent?.conflicts.contentBase
        if (conflict) {
          // The editor shows their clean hunks with our side of each
          // conflict, and the base is their body: the next save overwrites
          // only the regions that conflicted.
          if (contentChannelEnabled)
            opts.applyServerContent(sent?.conflicts.contentShown ?? entity.content)
          contentBase = conflict
        } else {
          if (contentChannelEnabled) opts.applyServerContent(entity.content)
          if (versions) contentBase = { value: entity.content, token: versions.content }
        }
      } else if (sent?.written.content && !sent.mergedContent && versions) {
        // The user kept typing while this save was in flight; their text
        // derives from what was saved, so that is the base.
        contentBase = { value: entity.content, token: versions.content }
      }
    }
    // The form never takes relations from a response, so the base moves
    // only when this response answers an unmerged relations save: then the
    // form's lists are exactly what the server stores.
    if (sent?.written.relations && !sent.mergedRelations && versions && entity.relations) {
      relationsBase = { value: { ...entity.relations }, token: versions.relations }
    }
  }

  // Drop a not-yet-sent write for `property` WITHOUT touching form state.
  //
  // `revertField` also restores `lastSeenServer` into the form, which is the
  // wrong baseline when the caller already knows the exact value to restore
  // (it can be older than an intermediate edit the user accepted). Callers
  // that own the restore need only the cancellation half. Returns true if a
  // pending write was dropped; false means it had already fired and the caller
  // must re-save.
  //
  // Currently exercised only by tests: its consumer was the interactive
  // clear-confirm path, deferred to the propose/commit refactor (BUG-FB0LN8).
  // Kept because "cancel a staged write without side effects" is the primitive
  // that refactor needs, and it is cheap and covered.
  function cancelPendingField(property: string): boolean {
    let cancelled = false
    if (timers[property]) {
      clearTimeout(timers[property])
      delete timers[property]
      cancelled = true
    }
    if (property in pending) {
      delete pending[property]
      pendingCount.value = Math.max(0, pendingCount.value - 1)
      cancelled = true
    }
    return cancelled
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
    //
    // fireDue drains every ripe property in one batch, so the FIRST iteration
    // typically consumes them all and the rest no-op on its `!pending[p]`
    // guard. The loop is kept (over a key snapshot, so mutation during
    // iteration is safe) because a property enqueued by a merge callback would
    // otherwise be missed.
    for (const p of Object.keys(timers)) {
      const t = timers[p]
      if (t) clearTimeout(t)
      fireDue(p, true)
    }
    if (contentTimer) {
      clearTimeout(contentTimer)
      contentTimer = null
      fireContent()
    }
    if (relationsTimer || relationsDirty) {
      if (relationsTimer) {
        clearTimeout(relationsTimer)
        relationsTimer = null
      }
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
    cancelPendingField,
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

// preconditionConflicts returns the failed fields of a 412 answering a PATCH
// with preconditions, or null for any other error.
function preconditionConflicts(err: unknown): FieldConflicts | null {
  if (!(err instanceof ApiError) || err.status !== 412) return null
  return err.problem?.conflicts ?? null
}

function isEmptyConflicts(c: FieldConflicts): boolean {
  return !Object.keys(c.properties ?? {}).length && !c.content && !c.relations
}

function isEmptyPatch(p: EntityPatch): boolean {
  return (
    !Object.keys(p.properties ?? {}).length &&
    !(p.properties_unset ?? []).length &&
    p.content === undefined &&
    !p.relations
  )
}

function writtenBy(p: EntityPatch): SendResult['written'] {
  return {
    props: new Set([...Object.keys(p.properties ?? {}), ...(p.properties_unset ?? [])]),
    content: p.content !== undefined,
    relations: !!p.relations,
  }
}

// Spread concurrent retries apart so two tabs that lost the same race do
// not collide again on the resend.
function backoff(attempt: number, signal: AbortSignal): Promise<void> {
  const ms = 50 * attempt + Math.random() * 100
  return new Promise((resolve, reject) => {
    if (signal.aborted) return reject(signal.reason)
    const timer = setTimeout(resolve, ms)
    signal.addEventListener(
      'abort',
      () => {
        clearTimeout(timer)
        reject(signal.reason)
      },
      { once: true }
    )
  })
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
