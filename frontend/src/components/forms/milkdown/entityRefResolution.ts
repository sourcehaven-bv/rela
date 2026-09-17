/**
 * Applying resolved titles to `entityRef` nodes in the document.
 *
 * The `entityRef` node stores only the entity ID, because that is all the
 * markdown holds. Titles come from the server's `mentions` map, which is
 * computed per principal through `visibility.Reader` (BUG-R9EHKV). The editor
 * must not derive titles any other way: an entity the principal may not read
 * has no mention at all, and one whose display title is redacted arrives
 * flagged `inaccessible` with the ID as its title. Re-deriving either locally
 * would leak what the server deliberately withheld.
 *
 * So this module takes an `EntityRefResolver` — the same seam the rendered
 * view uses in `utils/markdown.ts` — and copies its answers onto the nodes as
 * view-only attributes. Nothing here writes to the markdown; `toMarkdown`
 * serializes the ID alone.
 */
import { $prose } from '@milkdown/kit/utils'
import { Plugin, PluginKey } from '@milkdown/kit/prose/state'
import type { EditorState, Transaction } from '@milkdown/kit/prose/state'
import type { Node } from '@milkdown/kit/prose/model'
import type { EntityRefResolver } from '@/utils/markdown'

/** Holds the resolver for the lifetime of one editor instance. */
export interface ResolverHandle {
  /** Swapped when the mentions map arrives or changes. */
  resolver: EntityRefResolver | undefined
}

export const entityRefResolutionKey = new PluginKey('rela-entity-ref-resolution')

/**
 * Attributes a resolver contributes to an `entityRef` node.
 *
 * `title` stays null when the resolver has no answer. `toDOM` then falls back
 * to showing the raw ID, which is the honest rendering for a reference that is
 * unresolved, unreadable, or points at nothing.
 */
interface ResolvedAttrs {
  title: string | null
  entityType: string | null
  inaccessible: boolean
  resolvedFromServer: boolean
}

/**
 * What the node already carries, used when there is no map to consult.
 *
 * A reference the user just inserted is not in the mentions map: that map was
 * computed server-side when the document loaded, and it cannot contain an ID
 * chosen afterwards. The picker knew the title it displayed and put it on the
 * node, so keeping it is both correct and the only way the new reference reads
 * as anything but a bare ID until the next load.
 *
 * This does not route around the read gate. The title being kept is one the
 * server already sent this principal, in the search response that populated
 * the picker. Nothing is derived locally.
 */
function keepExisting(node: Node): ResolvedAttrs {
  return {
    title: (node.attrs.title as string | null) ?? null,
    entityType: (node.attrs.entityType as string | null) ?? null,
    inaccessible: node.attrs.inaccessible === true,
    resolvedFromServer: node.attrs.resolvedFromServer === true,
  }
}

/**
 * The bare-ID rendering, used when the server had a map and left this ID out.
 *
 * Distinct from `keepExisting` on purpose. `makeRefResolver` returns undefined
 * when a response carried no mention data at all, and a resolver that answers
 * null when the server considered an ID and declined to resolve it. Treating
 * those the same let a title outlive the grant that produced it: a reference
 * resolved on one load, its read access revoked, then a reload whose map omits
 * it — the node kept the old title and went on displaying it.
 */
function clearToId(): ResolvedAttrs {
  return { title: null, entityType: null, inaccessible: false, resolvedFromServer: false }
}

function resolveAttrs(
  node: Node,
  id: string,
  resolve: EntityRefResolver | undefined
): ResolvedAttrs {
  // No map at all: this response carried no mention data, so there is nothing
  // to contradict what the node holds.
  if (!resolve) return keepExisting(node)
  let hit
  try {
    hit = resolve(id)
  } catch {
    // A throwing resolver is treated as "no map", matching
    // rewriteEntityRefToken. The reference keeps whatever it had.
    return keepExisting(node)
  }
  // The server had a map and this ID is not in it: unreadable, nonexistent, or
  // inserted after the map was computed. A node the user just inserted has not
  // been through a reload yet, so it is the one case where keeping the
  // picker's title is right; anything already resolved must fall back to the
  // ID rather than show a title the server has stopped vouching for.
  if (!hit || !hit.type) {
    return node.attrs.resolvedFromServer === true ? clearToId() : keepExisting(node)
  }
  return {
    // An inaccessible target's title is already the ID server-side; keep the
    // fallback here too so a future wire change cannot turn a blank title into
    // a blank link.
    title: hit.title || null,
    entityType: hit.type,
    inaccessible: hit.inaccessible === true,
    resolvedFromServer: true,
  }
}

function attrsDiffer(node: Node, next: ResolvedAttrs): boolean {
  return (
    node.attrs.title !== next.title ||
    node.attrs.entityType !== next.entityType ||
    node.attrs.inaccessible !== next.inaccessible ||
    // Included so the first successful resolution of a picker-inserted
    // reference records its provenance, even when the title it confirms is
    // the one the picker already showed.
    node.attrs.resolvedFromServer !== next.resolvedFromServer
  )
}

/**
 * Builds a transaction that brings every `entityRef` node's view attributes in
 * line with the resolver, or null when nothing needs to change.
 *
 * Exported so the resolution logic can be tested without an editor view.
 */
export function buildResolutionTransaction(
  state: EditorState,
  resolve: EntityRefResolver | undefined
): Transaction | null {
  const updates: Array<{ pos: number; attrs: Record<string, unknown> }> = []
  state.doc.descendants((node, pos) => {
    if (node.type.name !== 'entityRef') return
    const next = resolveAttrs(node, node.attrs.id as string, resolve)
    if (!attrsDiffer(node, next)) return
    updates.push({ pos, attrs: { ...node.attrs, ...next } })
  })
  if (updates.length === 0) return null

  const tr = state.tr
  for (const { pos, attrs } of updates) {
    tr.setNodeMarkup(pos, undefined, attrs)
  }
  // The document's meaning is unchanged, so this must not enter the undo
  // history and must not mark the editor dirty: a title arriving from the
  // server is not an edit the user made. `addToHistory: false` keeps it out of
  // undo, the meta flag lets the change listener ignore it, and serialization
  // drops these attributes entirely.
  tr.setMeta('addToHistory', false)
  tr.setMeta(entityRefResolutionKey, true)
  return tr
}

/**
 * Reports whether a transaction is one this module produced, so a change
 * listener can ignore it rather than treat it as a user edit.
 */
export function isResolutionTransaction(tr: Transaction): boolean {
  return tr.getMeta(entityRefResolutionKey) === true
}

/**
 * The ProseMirror plugin. Re-resolves on every document change so a reference
 * pasted or typed after load also picks up its title.
 */
export const entityRefResolutionPlugin = (handle: ResolverHandle) =>
  $prose(
    () =>
      new Plugin({
        key: entityRefResolutionKey,
        appendTransaction: (transactions, _oldState, newState) => {
          // Only react to changes that touched the document; a bare selection
          // move cannot have introduced an unresolved reference.
          if (!transactions.some((tr) => tr.docChanged)) return null
          return buildResolutionTransaction(newState, handle.resolver)
        },
      })
  )
