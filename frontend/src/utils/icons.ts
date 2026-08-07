/** Icon registry — the ONLY place a config string becomes an icon component.
 *
 * Icon names arrive from project-authored data-entry.yaml (navigation entries,
 * kanban columns/swimlanes). Resolution is a STATIC ALLOWLIST lookup, never
 * dynamic component resolution from the string: a config value must never be
 * able to name an arbitrary component. Unknown names fall back to a default
 * rather than throwing, so a stale or hand-edited config still renders — the
 * server rejects unknown names at load, which is where an author gets told.
 *
 * Icons are Lucide (ISC), imported by name so the bundler tree-shakes the rest.
 * They render as inline SVG with `stroke="currentColor"`, which is the whole
 * point of the migration: the emoji they replace could not take the theme's
 * colour, rendered differently per OS, and sat on an inconsistent baseline.
 *
 * Adding an icon: import it, add one entry to ICONS, and add the same name to
 * the Go allowlist in internal/dataentryconfig (validIconNames). The two lists
 * are pinned to each other by a test — a name the SPA can render but the config
 * validator rejects (or vice versa) is a broken contract in either direction.
 */
import type { Component } from 'vue'
import {
  AlertTriangle,
  Blocks,
  CheckCircle2,
  CircleDot,
  Clock,
  FileText,
  Home,
  Inbox,
  Kanban,
  List,
  Moon,
  Search,
  Settings,
  Sun,
  Wrench,
} from 'lucide-vue-next'

/** DEFAULT_ICON renders for any name not in the allowlist. */
export const DEFAULT_ICON: Component = FileText

/** ICONS maps a config-facing name to its component. Keys are the public
 * contract — renaming one breaks every project that authored it. */
export const ICONS: Record<string, Component> = {
  // Navigation
  dashboard: Home,
  list: List,
  kanban: Kanban,
  search: Search,
  analysis: AlertTriangle,
  apps: Blocks,
  settings: Settings,
  document: FileText,
  // Theme toggle
  sun: Sun,
  moon: Moon,
  // Workflow-ish names, useful for kanban columns
  inbox: Inbox,
  progress: Wrench,
  done: CheckCircle2,
  clock: Clock,
  status: CircleDot,
}

/** iconNames lists every valid name, for tests and error messages. */
export const iconNames = (): string[] => Object.keys(ICONS).sort()

/**
 * resolveIcon returns the component for a config-supplied name.
 *
 * Falls back to DEFAULT_ICON for an unknown or absent name — never throws and
 * never returns undefined, so a caller can bind the result straight to
 * `<component :is>` without a guard.
 */
export function resolveIcon(name?: string | null): Component {
  // isKnownIcon, not `ICONS[name] ?? DEFAULT_ICON`: a bare index lookup finds
  // INHERITED Object.prototype members, so a config naming `toString` or
  // `constructor` would yield a function that is not a component and blow up
  // the render. An own-property check is the difference between a fallback
  // icon and a crash.
  if (!isKnownIcon(name)) return DEFAULT_ICON
  return ICONS[name]
}

/** isKnownIcon reports whether a name resolves to a real icon (not the
 * fallback). A type predicate so callers can index ICONS afterwards. Used by
 * tests; the server is what validates author input. */
export function isKnownIcon(name?: string | null): name is string {
  // Not Object.hasOwn — the project's TS lib target predates it.
  return !!name && Object.prototype.hasOwnProperty.call(ICONS, name)
}
