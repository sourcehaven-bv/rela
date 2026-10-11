import { defineStore } from 'pinia'
import { computed, ref, shallowRef } from 'vue'
import type {
  ConfigDraftBody,
  ConfigProblem,
  ConfigRename,
  ConfigResult,
  ConfigSnapshot,
  ConfigValueMapping,
} from '@/api/configure'
import { relaBase } from '@/api/base'
import { changeCount, describeChanges, type ChangeGroup } from '@/configure/changes'
import { renameSchemaReferences, renameScreenReferences } from '@/configure/references'
import {
  clone,
  ensureList,
  ensurePath,
  equal,
  get,
  getIn,
  has,
  isList,
  isMap,
  keys,
  listAt,
  mapAt,
  moveItem,
  newMap,
  remove,
  renameKey,
  set,
  type TreeMap,
  type TreePath,
  type TreeValue,
} from '@/configure/tree'

/**
 * The Configure space's unsaved changes (TKT-F5NGMG).
 *
 * The server's configuration (the base) is never edited. The first edit to a
 * file copies its tree, and the copy is the draft; a copy that edits make
 * equal to the base again is dropped, so undoing a change by hand leaves
 * nothing to save. Beside the trees the draft holds what a tree cannot say:
 * that a property was renamed rather than replaced, and where records holding
 * a removed option should go.
 *
 * The draft is kept in localStorage per project, so a reload or a closed tab
 * loses nothing. Every tab of the project shares that one copy: an edit in
 * one tab is picked up by the others through the `storage` event, so the
 * last edit wins and every tab shows it. It remembers the version it was
 * made against; when the server's version has moved on, saving would be
 * refused, so the screens say so and offer to discard.
 */

/** What is kept in localStorage. */
interface StoredDraft {
  base_version: string
  schema?: TreeValue
  data_entry?: TreeValue
  renames?: ConfigRename[]
  value_mappings?: ConfigValueMapping[]
  migration_title?: string
}

const MIGRATION_TITLE_MAX_BYTES = 120

export function draftStorageKey(): string {
  return `rela:configure-draft:${relaBase() || '/'}`
}

function parseStored(raw: string | null): StoredDraft | null {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as StoredDraft
    return typeof parsed?.base_version === 'string' ? parsed : null
  } catch {
    return null
  }
}

function readStored(): StoredDraft | null {
  try {
    return parseStored(localStorage.getItem(draftStorageKey()))
  } catch {
    return null
  }
}

/** Writes the draft; false when storage is full or refused. */
function writeStored(draft: StoredDraft | null): boolean {
  try {
    if (draft) localStorage.setItem(draftStorageKey(), JSON.stringify(draft))
    else localStorage.removeItem(draftStorageKey())
    return true
  } catch {
    return false
  }
}

/** The longest start of `text` that fits in `maxBytes` of UTF-8, whole code points only. */
export function truncateUtf8(text: string, maxBytes: number): string {
  const encoder = new TextEncoder()
  if (encoder.encode(text).length <= maxBytes) return text
  let out = ''
  let bytes = 0
  for (const char of text) {
    const size = encoder.encode(char).length
    if (bytes + size > maxBytes) break
    out += char
    bytes += size
  }
  return out
}

/**
 * The base's value at `path` in a draft tree. A list item is matched to its
 * base item by `$i`, not by position, so a reordered item still finds its
 * original; a new item has none.
 */
function baseAt(
  tree: TreeValue,
  baseTree: TreeValue | undefined,
  path: TreePath
): TreeValue | undefined {
  let node: TreeValue | undefined = tree
  let baseNode = baseTree
  for (const segment of path) {
    if (typeof segment === 'number') {
      node = isList(node) ? node[segment] : undefined
      const origin = isMap(node) ? node.$i : undefined
      baseNode = origin !== undefined && isList(baseNode) ? baseNode[origin] : undefined
    } else {
      node = get(node, segment)
      baseNode = get(baseNode, segment)
    }
    if (baseNode === undefined) return undefined
  }
  return baseNode
}

export const useConfigDraftStore = defineStore('configDraft', () => {
  const base = shallowRef<ConfigSnapshot | null>(null)
  /** The edited copy of each file, or null while it is unedited. */
  const schema = shallowRef<TreeValue | null>(null)
  const dataEntry = shallowRef<TreeValue | null>(null)
  const renames = ref<ConfigRename[]>([])
  const valueMappings = ref<ConfigValueMapping[]>([])
  const migrationTitle = ref('')
  /** The version the draft was made against. */
  const draftVersion = ref<string | null>(null)
  /** The last preview or refused save, for the problems it reported. */
  const result = shallowRef<ConfigResult | null>(null)
  const reviewOpen = ref(false)
  /**
   * Counts changes to what the draft sends for checking, so an answer to an
   * older draft can be told apart from one to this draft.
   */
  const revision = ref(0)
  /**
   * Whether localStorage holds what memory holds. A refused write leaves an
   * older copy there, which must never replace the newer draft in memory.
   */
  let storedInSync = true

  const loaded = computed(() => base.value !== null)
  const currentSchema = computed<TreeValue | undefined>(() => schema.value ?? base.value?.schema)
  const currentDataEntry = computed<TreeValue | undefined>(
    () => dataEntry.value ?? base.value?.data_entry
  )
  const editable = computed(() => base.value?.editable)

  const hasDraft = computed(
    () =>
      schema.value !== null ||
      dataEntry.value !== null ||
      renames.value.length > 0 ||
      valueMappings.value.length > 0
  )

  /** The draft was made against a configuration that has since changed. */
  const stale = computed(
    () => hasDraft.value && base.value !== null && draftVersion.value !== base.value.version
  )

  const changes = computed<ChangeGroup[]>(() => {
    if (!base.value || (schema.value === null && dataEntry.value === null)) return []
    return describeChanges(
      { schema: base.value.schema, dataEntry: base.value.data_entry },
      { schema: currentSchema.value, dataEntry: currentDataEntry.value },
      renames.value
    )
  })

  const count = computed(() => changeCount(changes.value))
  const problems = computed<ConfigProblem[]>(() => result.value?.problems ?? [])

  function persist(): void {
    if (!base.value || !hasDraft.value) {
      writeStored(null)
      return
    }
    const stored: StoredDraft = { base_version: draftVersion.value ?? base.value.version }
    if (schema.value !== null) stored.schema = schema.value
    if (dataEntry.value !== null) stored.data_entry = dataEntry.value
    if (renames.value.length) stored.renames = renames.value
    if (valueMappings.value.length) stored.value_mappings = valueMappings.value
    if (migrationTitle.value) stored.migration_title = migrationTitle.value
    storedInSync = writeStored(stored)
  }

  /** Replaces the draft in memory with a stored one, or with nothing. */
  function applyStored(stored: StoredDraft | null, snapshot: ConfigSnapshot): void {
    schema.value = stored?.schema ?? null
    dataEntry.value = stored?.data_entry ?? null
    renames.value = stored?.renames ?? []
    valueMappings.value = stored?.value_mappings ?? []
    migrationTitle.value = stored?.migration_title ?? ''
    draftVersion.value = stored?.base_version ?? snapshot.version
    result.value = null
    revision.value++
  }

  /** Drops a file copy equal to the configuration, and the version of an empty draft. */
  function prune(snapshot: ConfigSnapshot): void {
    if (schema.value !== null && equal(schema.value, snapshot.schema)) schema.value = null
    if (dataEntry.value !== null && equal(dataEntry.value, snapshot.data_entry))
      dataEntry.value = null
    if (!hasDraft.value) draftVersion.value = snapshot.version
  }

  /** Takes the server's configuration, and picks up a draft kept from before. */
  function load(snapshot: ConfigSnapshot): void {
    const keepMemory = base.value !== null && !storedInSync
    base.value = snapshot
    if (keepMemory) {
      result.value = null
      revision.value++
    } else applyStored(readStored(), snapshot)
    // A kept draft equal to the configuration is nothing to keep.
    prune(snapshot)
    persist()
  }

  /** Another tab wrote the shared draft: show its version here too. */
  function onStorage(event: StorageEvent): void {
    if (event.key !== null && event.key !== draftStorageKey()) return
    const snapshot = base.value
    if (!snapshot) return
    applyStored(event.key === null ? null : parseStored(event.newValue), snapshot)
    prune(snapshot)
    storedInSync = true
  }

  if (typeof window !== 'undefined') window.addEventListener('storage', onStorage)

  function touched(): void {
    if (draftVersion.value === null || !hasDraft.value)
      draftVersion.value = base.value?.version ?? null
    result.value = null
    revision.value++
    persist()
  }

  function editTree(
    current: TreeValue | null,
    baseTree: TreeValue | undefined,
    fn: (tree: TreeMap) => void
  ): TreeValue | null {
    const working = clone(current ?? baseTree ?? newMap())
    if (!isMap(working)) throw new Error('configure: a configuration file must be a mapping')
    fn(working)
    return equal(working, baseTree) ? null : working
  }

  /**
   * Changes one file, the data model (`schema`) or the screens (`screens`).
   * `fn` edits a copy in place.
   */
  function edit(file: 'schema' | 'screens', fn: (tree: TreeMap) => void): void {
    if (!base.value) return
    if (file === 'schema') schema.value = editTree(schema.value, base.value.schema, fn)
    else dataEntry.value = editTree(dataEntry.value, base.value.data_entry, fn)
    touched()
  }

  /**
   * Sets or removes one setting of the mapping at `path`, creating the path if
   * needed. An empty text, null or undefined removes the setting; `false`
   * removes it only where the configuration did not have it, so an unticked
   * box that was ticked in the configuration is saved as an explicit `false`.
   * Removing the last setting of a mapping the configuration did not have
   * removes that mapping too, so undoing an edit leaves nothing behind.
   */
  function setAt(
    file: 'schema' | 'screens',
    path: TreePath,
    key: string,
    value: TreeValue | undefined
  ): void {
    const baseTree = file === 'schema' ? base.value?.schema : base.value?.data_entry
    edit(file, (tree) => {
      const original = get(baseAt(tree, baseTree, path), key)
      const removing =
        value === undefined ||
        value === '' ||
        value === null ||
        (value === false && original === undefined)
      const map = removing ? mapAt(tree, path) : ensurePath(tree, path)
      if (!map) return
      if (!removing) {
        set(map, key, value)
        return
      }
      remove(map, key)
      for (let depth = path.length; depth > 0; depth--) {
        const at = path.slice(0, depth)
        const last = at[at.length - 1]
        const parent = mapAt(tree, at.slice(0, -1))
        if (typeof last !== 'string' || !parent || keys(mapAt(tree, at)).length) break
        if (baseAt(tree, baseTree, at) !== undefined) break
        remove(parent, last)
      }
    })
  }

  /** Moves an item of the list at `path`. */
  function moveAt(file: 'schema' | 'screens', path: TreePath, from: number, to: number): void {
    edit(file, (tree) => {
      const list = listAt(tree, path)
      if (list && from >= 0 && from < list.length) moveItem(list, list[from], to)
    })
  }

  /** Removes an item of the list at `path`. */
  function removeAt(file: 'schema' | 'screens', path: TreePath, index: number): void {
    edit(file, (tree) => {
      const list = listAt(tree, path)
      if (list && index >= 0 && index < list.length) list.splice(index, 1)
    })
  }

  /**
   * Removes an item of the list under `key` of the mapping at `path`, and the
   * key itself once the list is empty, so no `key: []` is left behind.
   */
  function removeItemAt(
    file: 'schema' | 'screens',
    path: TreePath,
    key: string,
    index: number
  ): void {
    edit(file, (tree) => {
      const map = mapAt(tree, path)
      const list = listAt(map, [key])
      if (!map || !list || index < 0 || index >= list.length) return
      list.splice(index, 1)
      if (!list.length) remove(map, key)
    })
  }

  /** Appends an item to the list under `key` of the mapping at `path`. */
  function appendAt(
    file: 'schema' | 'screens',
    path: TreePath,
    key: string,
    item: TreeValue
  ): void {
    edit(file, (tree) => {
      const map = ensurePath(tree, path)
      if (map) ensureList(map, key).push(item)
    })
  }

  /** The name a draft property had in the configuration, if it had one. */
  function originalName(entityType: string, name: string): string | undefined {
    const renamed = renames.value.find((r) => r.entity_type === entityType && r.to === name)
    const original = renamed ? renamed.from : name
    return has(getIn(base.value?.schema, ['entities', entityType, 'properties']), original)
      ? original
      : undefined
  }

  /**
   * Renames a property. The values records hold move with it, and the forms,
   * lists and boards that show it follow. Renaming twice records one rename;
   * renaming back records none.
   */
  function renameProperty(entityType: string, from: string, to: string): void {
    if (from === to || !base.value) return
    const original = originalName(entityType, from)
    edit('schema', (tree) => {
      const props = getIn(tree, ['entities', entityType, 'properties'])
      if (!isMap(props)) return
      renameKey(props, from, to)
      renameSchemaReferences(tree, entityType, from, to)
    })
    edit('screens', (tree) => renameScreenReferences(tree, entityType, from, to))
    const rest = renames.value.filter((r) => !(r.entity_type === entityType && r.to === from))
    if (original !== undefined && original !== to)
      rest.push({ entity_type: entityType, from: original, to })
    renames.value = rest
    // A value mapping names the property as it is after the save.
    valueMappings.value = valueMappings.value.map((m) =>
      m.entity_type === entityType && m.property === from ? { ...m, property: to } : m
    )
    touched()
  }

  /** Forgets the rename of a property that is removed. */
  function forgetProperty(entityType: string, name: string): void {
    renames.value = renames.value.filter((r) => !(r.entity_type === entityType && r.to === name))
    valueMappings.value = valueMappings.value.filter(
      (m) => !(m.entity_type === entityType && m.property === name)
    )
    touched()
  }

  function mappingFor(
    entityType: string,
    property: string,
    from: string
  ): ConfigValueMapping | undefined {
    return valueMappings.value.find(
      (m) => m.entity_type === entityType && m.property === property && m.from === from
    )
  }

  /** Says where records holding a removed option go. An empty `to` forgets it. */
  function setValueMapping(mapping: ConfigValueMapping): void {
    const rest = valueMappings.value.filter(
      (m) =>
        !(
          m.entity_type === mapping.entity_type &&
          m.property === mapping.property &&
          m.from === mapping.from
        )
    )
    if (mapping.to) rest.push(mapping)
    valueMappings.value = rest
    touched()
  }

  function setMigrationTitle(title: string): void {
    migrationTitle.value = title
    persist()
  }

  /** Drops every unsaved change. */
  function discardAll(): void {
    schema.value = null
    dataEntry.value = null
    renames.value = []
    valueMappings.value = []
    migrationTitle.value = ''
    draftVersion.value = base.value?.version ?? null
    result.value = null
    revision.value++
    reviewOpen.value = false
    storedInSync = writeStored(null)
  }

  /** The draft as the server takes it. */
  function body(): ConfigDraftBody {
    const out: ConfigDraftBody = { base_version: draftVersion.value ?? base.value?.version ?? '' }
    if (schema.value !== null) out.schema = schema.value
    if (dataEntry.value !== null) out.data_entry = dataEntry.value
    if (renames.value.length) out.renames = renames.value
    if (valueMappings.value.length) out.value_mappings = valueMappings.value
    const title = migrationTitle.value.trim()
    // The server's limit is in bytes, not characters.
    if (title) out.migration_title = truncateUtf8(title, MIGRATION_TITLE_MAX_BYTES).trimEnd()
    return out
  }

  function setResult(next: ConfigResult | null): void {
    result.value = next
  }

  return {
    base,
    schema,
    dataEntry,
    renames,
    valueMappings,
    migrationTitle,
    draftVersion,
    result,
    reviewOpen,
    revision,
    loaded,
    currentSchema,
    currentDataEntry,
    editable,
    hasDraft,
    stale,
    changes,
    count,
    problems,
    load,
    edit,
    setAt,
    moveAt,
    removeAt,
    removeItemAt,
    appendAt,
    renameProperty,
    forgetProperty,
    originalName,
    mappingFor,
    setValueMapping,
    setMigrationTitle,
    discardAll,
    body,
    setResult,
  }
})
