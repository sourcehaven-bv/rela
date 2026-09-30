import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { SidebarCreate, SidebarSpace } from '@/types'

/**
 * Matches a space prefix: `/s/<id>` followed by the rest of the path.
 *
 * The id pattern is the one the server validates `spaces[].id` against, so a
 * path that matches here names a space the config could declare.
 */
const SPACE_PATH = /^\/s\/([a-z][a-z0-9_-]{0,31})(\/.*)?$/

/** The space id a path is prefixed with, or undefined for an unprefixed path. */
export function spaceOf(path: string): string | undefined {
  return SPACE_PATH.exec(path)?.[1]
}

/** A path without its space prefix. An unprefixed path is returned as is. */
export function stripSpace(path: string): string {
  const m = SPACE_PATH.exec(path)
  if (!m) return path
  return m[2] || '/'
}

/**
 * A path under a space. Leaves external URLs, already-prefixed paths and API
 * paths alone, so it is safe to apply to any href the config produced.
 */
export function withSpace(path: string, space: string | undefined): string {
  if (!space || !path.startsWith('/') || path.startsWith('//')) return path
  if (spaceOf(path) || path.startsWith('/api/')) return path
  return `/s/${space}${path === '/' ? '' : path}`
}

/**
 * The spaces this principal may enter, and the one they are in (TKT-GNKR5H).
 *
 * Fed by the `_sidebar` response, which resolves the current space on the
 * server: an unknown or hidden space falls back to the first one the principal
 * may enter. Without `spaces:` in the config the list is empty and every path
 * stays unprefixed, exactly as before spaces existed.
 */
export const useSpaceStore = defineStore('space', () => {
  const spaces = ref<SidebarSpace[]>([])
  const current = ref<string | undefined>(undefined)
  const create = ref<SidebarCreate[]>([])

  const enabled = computed(() => spaces.value.length > 0)
  const currentSpace = computed(() => spaces.value.find((s) => s.id === current.value))

  function set(data: { spaces?: SidebarSpace[]; space?: string; create?: SidebarCreate[] }) {
    spaces.value = data.spaces ?? []
    current.value = data.space || undefined
    create.value = data.create ?? []
  }

  /** The href of a path in the current space. */
  function href(path: string): string {
    return enabled.value ? withSpace(path, current.value) : path
  }

  return { spaces, current, create, enabled, currentSpace, set, href }
})
