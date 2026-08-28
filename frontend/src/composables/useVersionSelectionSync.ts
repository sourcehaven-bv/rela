/**
 * Bidirectional sync between the history views' compared-version pair and the
 * `?base=` / `?target=` query params, so a specific diff is a shareable link.
 *
 * Mirrors the seed/replace/echo pattern of `useUrlFilterSync` and
 * `useFormWizard`'s `?step=N`:
 *
 * - Seeds synchronously at setup so a deep link is honoured as early as
 *   possible, and is re-runnable (`seedFromUrl`) because the set of valid
 *   versions isn't known until `listVersions` resolves — the same reason
 *   useFormWizard re-seeds after its entity loads.
 * - `select` is the single write path: it updates the refs and the URL via
 *   `router.replace` (never `push`, so fiddling with the dropdowns doesn't bury
 *   the Back button under one entry per change) and records a signature the
 *   route watcher uses to ignore its own echo.
 * - The route watcher handles external navigation (back/forward, a pasted
 *   link) and re-seeds from the query.
 *
 * A param is accepted only if it is the literal `current` or an ordinal the
 * server actually returned. That allowlist — rather than a numeric range check
 * — is what keeps a crafted `?base=../../x` or `?base=999` from ever reaching a
 * fetch: an unrecognized value silently becomes the view's default.
 */
import { ref, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

/**
 * A comparison side: a version ordinal, or the live/newest state.
 *
 * `current` is spelled the same in the URL for both history views even though
 * they resolve and label it differently — the entity view diffs against the
 * live entity ("current"), while the relation view has no live-fetch endpoint
 * and maps it to the newest snapshot ("latest"). Keeping one URL token means a
 * link reads the same either way.
 */
export type Side = number | 'current'

/** The sentinel's URL spelling, shared by both views. */
export const CURRENT = 'current'

export interface VersionSelection {
  base: Side
  target: Side
}

export interface UseVersionSelectionSyncOptions {
  /**
   * Version ordinals that actually exist. Supplied by the view, and empty
   * until `listVersions` resolves — which is precisely why seeding has to be
   * re-runnable rather than a one-shot at setup.
   */
  validVersions: () => number[]
  /**
   * The view's default pair, used for any side the URL doesn't validly
   * specify. The two views disagree (entity: newest → current; relation:
   * second-newest → newest), so this can't be hardcoded here.
   */
  defaults: () => VersionSelection
  /** Run after any change to the pair, from any source. */
  onChange: () => void
}

/** Collapse a query value to a single string, taking the last of a repeated
 *  param (`?base=1&base=2`) the way `useUrlFilterSync.readQParam` does. */
function readParam(value: unknown): string {
  if (typeof value === 'string') return value
  if (Array.isArray(value)) {
    const last = value[value.length - 1]
    return typeof last === 'string' ? last : ''
  }
  return ''
}

/**
 * Resolve one raw query value to a Side, or null if it names nothing real.
 *
 * The numeric branch must produce a `number`, not a numeric string: the
 * `<option value="current">` is a string while version options bind
 * `:value="m.version"` (a number), so a string `'3'` would match no option and
 * `v-model` would render the dropdown blank.
 */
export function parseSide(raw: unknown, validVersions: number[]): Side | null {
  const value = readParam(raw)
  if (value === CURRENT) return CURRENT
  if (!/^\d+$/.test(value)) return null // rejects '', 'abc', '-1', '1.5', '1e2'
  const n = Number(value)
  return validVersions.includes(n) ? n : null
}

/** Serialize a Side for the URL. */
export function serializeSide(s: Side): string {
  return s === CURRENT ? CURRENT : String(s)
}

/**
 * Normalize a Side to the numeric form the dropdowns bind.
 *
 * `parseSide` already guarantees this for URL-sourced values, but sides also
 * arrive from callers (a view's `defaults()`, a `select()` argument). Coercing
 * on every write keeps the invariant with the type instead of relying on each
 * caller — a numeric STRING would match no `<option>` and silently blank the
 * control, since version options bind numbers while the sentinel binds a string.
 */
export function coerceSide(s: Side): Side {
  return s === CURRENT ? CURRENT : Number(s)
}

export function useVersionSelectionSync(opts: UseVersionSelectionSyncOptions) {
  const route = useRoute()
  const router = useRouter()

  const base = ref<Side>(CURRENT) as Ref<Side>
  const target = ref<Side>(CURRENT) as Ref<Side>

  // Signature of the pair we last wrote ourselves, so the route watcher can
  // tell our own `router.replace` echo from real external navigation.
  let lastWrittenSig = ''

  function signature(b: Side, t: Side): string {
    return `${serializeSide(b)}|${serializeSide(t)}`
  }

  /**
   * Read the pair from the URL, falling back per-side to the view's defaults.
   * Re-runnable: call it again once the version list has loaded so params can
   * be validated against real ordinals.
   */
  function seedFromUrl(): void {
    const valid = opts.validVersions()
    const fallback = opts.defaults()
    base.value = parseSide(route.query.base, valid) ?? coerceSide(fallback.base)
    target.value = parseSide(route.query.target, valid) ?? coerceSide(fallback.target)
  }

  /**
   * Write the current pair to the URL, merging into the existing query so
   * unrelated params (`return_to`, `from`, …) survive.
   *
   * Exported as `publish` for the views to call after a load has already
   * computed the diff: it turns a bare or stale URL into an explicit, shareable
   * one. It does NOT recompute — `select` owns that.
   */
  function writeToQuery(): void {
    const query = {
      ...route.query,
      base: serializeSide(base.value),
      target: serializeSide(target.value),
    }
    lastWrittenSig = signature(base.value, target.value)
    void router.replace({ query })
  }

  /** The single sanctioned mutation path: set one or both sides, publish to
   *  the URL, and recompute. Every caller (dropdown, timeline row, swap) goes
   *  through here so no path can update the URL without recomputing the diff,
   *  or recompute without updating the URL. */
  function select(next: Partial<VersionSelection>): void {
    if (next.base !== undefined) base.value = coerceSide(next.base)
    if (next.target !== undefined) target.value = coerceSide(next.target)
    writeToQuery()
    opts.onChange()
  }

  /** Reset both sides to the view's defaults and publish, WITHOUT reading the
   *  URL. Used after a restore: the version list has changed underneath, so
   *  re-seeding would resurrect a pair the user picked against the old list.
   *  Exists so the views don't assign the refs directly and bypass the write
   *  path. */
  function resetToDefaults(): void {
    const d = opts.defaults()
    base.value = coerceSide(d.base)
    target.value = coerceSide(d.target)
  }

  /** Swap the two sides (reverse the diff direction). */
  function swap(): void {
    select({ base: target.value, target: base.value })
  }

  // External navigation (back/forward, a pasted link). Our own writes are
  // skipped via the signature guard, which is what keeps this from looping.
  //
  // The getter returns a joined STRING, not an array: an array getter allocates
  // a fresh reference every run, so Vue's reference comparison would fire the
  // watcher on any unrelated query change (a `?q=` edit elsewhere) and
  // spuriously recompute the diff.
  watch(
    () => `${readParam(route.query.base)}|${readParam(route.query.target)}`,
    () => {
      const valid = opts.validVersions()
      const fallback = opts.defaults()
      const incoming = signature(
        parseSide(route.query.base, valid) ?? fallback.base,
        parseSide(route.query.target, valid) ?? fallback.target
      )
      if (incoming === lastWrittenSig) return
      lastWrittenSig = incoming
      seedFromUrl()
      opts.onChange()
    }
  )

  // Seed once synchronously. The version list is still empty here, so this
  // only picks up `current`; seedFromUrl runs again after load resolves.
  seedFromUrl()

  return { base, target, seedFromUrl, resetToDefaults, select, swap, publish: writeToQuery }
}
