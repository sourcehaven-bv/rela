/**
 * Turning a task item into a new linked entity (TKT-3IBLIP).
 *
 * The operator declares `task_item_conversions:` recipes; the server lists the
 * ones a source entity offers. This module draws one button per task item and
 * applies the outcome to the document. It does no API work: the host listens
 * for the button, talks to the server, and hands the result back.
 *
 * The button is a widget decoration, not part of the task-item node view, so
 * offers that arrive after the editor mounted show up without rebuilding any
 * node. Like the checkbox it is chrome: built with `createElement` and
 * `textContent` only, `contentEditable=false`, and every event on it is
 * claimed so ProseMirror never places a caret inside it.
 */
import { $prose } from '@milkdown/kit/utils'
import { closeHistory } from '@milkdown/kit/prose/history'
import { Plugin, PluginKey, type EditorState, type Transaction } from '@milkdown/kit/prose/state'
import { Fragment, type Node as ProseNode } from '@milkdown/kit/prose/model'
import { Decoration, DecorationSet, type EditorView } from '@milkdown/kit/prose/view'
import { isTaskItem } from './taskListItem'
import { isValidEntityRefId } from '@/utils/entityRefId'
import type { EntityRefInsert } from './insertEntityRef'

/** A task item, named by its position and the text the server will get. */
export interface TaskItemTarget {
  pos: number
  text: string
}

/** What the host wires into the plugin. Mutated in place, like `ResolverHandle`. */
export interface TaskItemConvertHandle {
  /** The button's accessible label; empty hides every button. */
  label: string
  onConvert?: (target: TaskItemTarget, anchor: HTMLElement) => void
}

/** An item with a request in flight, named by the token its mark returned. */
interface BusyMark {
  token: number
  pos: number
}

interface TaskItemConvertState {
  /** The items with a request in flight; their buttons are disabled. */
  busy: BusyMark[]
  /** The buttons, rebuilt only when the document, the marks or the handle change. */
  decorations: DecorationSet
}

type TaskItemConvertMeta = { refresh: true } | { mark: BusyMark } | { clear: number }

let nextBusyToken = 1

export const taskItemConvertKey = new PluginKey<TaskItemConvertState>('rela-task-item-convert')

/**
 * The text of an item's own line: its first paragraph, with references as
 * their ids and whitespace collapsed. Nested items are not part of it.
 */
export function taskItemText(item: ProseNode): string {
  const para = item.firstChild
  if (!para || para.type.name !== 'paragraph') return ''
  const text = para.textBetween(0, para.content.size, ' ', (leaf) =>
    leaf.type.name === 'entityRef' ? String(leaf.attrs.id ?? '') : ' '
  )
  return text.replace(/\s+/g, ' ').trim()
}

/**
 * Whether the item's line holds anything besides entity references. An item
 * that is already only a reference has nothing left to convert.
 */
function hasOwnText(item: ProseNode): boolean {
  let own = false
  item.firstChild?.forEach((child) => {
    if (child.isText && child.text?.trim()) own = true
  })
  return own
}

/**
 * Finds the item a button press named, to mark it busy. The item at `pos`
 * wins if it still has the same text; otherwise the one task item with that
 * text. Several, or none, is null: guessing would mark a line the user did
 * not convert. From then on the mark, not this search, names the item.
 */
export function findTaskItem(doc: ProseNode, target: TaskItemTarget): number | null {
  const at = target.pos >= 0 && target.pos < doc.content.size ? doc.nodeAt(target.pos) : null
  if (at && isTaskItem(at) && taskItemText(at) === target.text) return target.pos
  const matches: number[] = []
  doc.descendants((node, pos) => {
    if (isTaskItem(node) && taskItemText(node) === target.text) matches.push(pos)
    return true
  })
  return matches.length === 1 ? matches[0] : null
}

/**
 * Applies a finished conversion to the item the busy mark `token` names, as
 * one undo step. `link` puts a reference to `ref` in place of the line's
 * plain text; `remove` deletes the item and lifts its nested items into its
 * place.
 *
 * The item is the one the mark followed through edits, never another item
 * with the same text. `text` only confirms it: an item the user edited while
 * the request ran no longer says what the server converted.
 *
 * `remove` links instead when removing would drop something the user wrote:
 * a block after the line other than a nested list (a second paragraph, a code
 * block), or a link or other inline node the item's text cannot carry.
 *
 * Returns false, and leaves the document alone, when the mark is gone, the
 * item changed or the reference is not valid; the caller then tells the user
 * the new id instead.
 */
export function applyTaskItemResult(
  view: EditorView,
  token: number,
  text: string,
  mode: 'link' | 'remove',
  ref: EntityRefInsert
): boolean {
  const { state } = view
  const mark = taskItemConvertKey.getState(state)?.busy.find((m) => m.token === token)
  const item = mark ? state.doc.nodeAt(mark.pos) : null
  if (!mark || !item || !isTaskItem(item) || taskItemText(item) !== text) return false
  const tr =
    mode === 'remove' && liftable(item) && flattens(item)
      ? removeItem(state, mark.pos, item)
      : linkItem(state, mark.pos, item, ref)
  if (!tr) return false
  closeHistory(tr)
  view.dispatch(tr)
  return true
}

function isList(node: ProseNode): boolean {
  return node.type.name === 'bullet_list' || node.type.name === 'ordered_list'
}

/** Whether every block after the item's line is a nested list. */
function liftable(item: ProseNode): boolean {
  let ok = true
  item.forEach((child, _offset, index) => {
    if (index > 0 && !isList(child)) ok = false
  })
  return ok
}

function hasLink(node: ProseNode): boolean {
  return node.marks.some((mark) => mark.type.name === 'link')
}

/**
 * Whether the line is plain text: text outside links, which the link mode
 * replaces. A hard break counts too, as the text has it as a space.
 */
function isPlain(node: ProseNode): boolean {
  return (node.isText && !hasLink(node)) || node.type.name === 'hardbreak'
}

/**
 * Whether the item's text, which the new entity's title came from, holds all
 * of its line. A reference is in it as its id; a link's target and an image
 * are not.
 */
function flattens(item: ProseNode): boolean {
  let ok = true
  item.firstChild?.forEach((child) => {
    if (!isPlain(child) && child.type.name !== 'entityRef') ok = false
  })
  return ok
}

/**
 * Puts the reference in place of the line's plain text. Linked text and other
 * inline nodes stay, after the reference; each run of them is set off by a
 * space, as the plain text between them is gone.
 */
function linkItem(
  state: EditorState,
  pos: number,
  item: ProseNode,
  ref: EntityRefInsert
): Transaction | null {
  const nodeType = state.schema.nodes.entityRef
  const para = item.firstChild
  if (!nodeType || !para || !isValidEntityRefId(ref.id)) return null
  const line: ProseNode[] = [
    nodeType.create({
      id: ref.id,
      title: ref.title || null,
      entityType: ref.entityType,
      inaccessible: false,
    }),
  ]
  let gap = true
  para.forEach((child) => {
    if (isPlain(child)) {
      gap = true
      return
    }
    if (gap) line.push(state.schema.text(' '))
    line.push(child)
    gap = false
  })
  // pos is before the item, pos + 1 before its paragraph, pos + 2 its content.
  const from = pos + 2
  return state.tr.replaceWith(from, from + para.content.size, Fragment.fromArray(line))
}

function removeItem(state: EditorState, pos: number, item: ProseNode): Transaction {
  const lifted: ProseNode[] = []
  item.forEach((child, _offset, index) => {
    if (index > 0) child.forEach((nested) => lifted.push(nested))
  })
  const $pos = state.doc.resolve(pos)
  const list = $pos.parent
  if (lifted.length === 0 && list.childCount === 1) {
    // The list would be empty; remove it rather than leave an empty bullet.
    return state.tr.delete($pos.before(), $pos.after())
  }
  return state.tr.replaceWith(pos, pos + item.nodeSize, Fragment.fromArray(lifted))
}

/**
 * Marks a request for an item as running and returns the token that clears
 * it, or null when the item cannot be found. The mark follows the item
 * through edits and goes with it when the item is deleted.
 */
export function markTaskItemBusy(view: EditorView, target: TaskItemTarget): number | null {
  const pos = findTaskItem(view.state.doc, target)
  if (pos == null) return null
  const meta: TaskItemConvertMeta = { mark: { token: nextBusyToken++, pos } }
  view.dispatch(view.state.tr.setMeta(taskItemConvertKey, meta))
  return meta.mark.token
}

/** Ends the mark `token` names. A mark that went with its item is a no-op. */
export function clearTaskItemBusy(view: EditorView, token: number): void {
  const meta: TaskItemConvertMeta = { clear: token }
  view.dispatch(view.state.tr.setMeta(taskItemConvertKey, meta))
}

/** Rebuilds the buttons after the handle changed. */
export function refreshTaskItemButtons(view: EditorView): void {
  const meta: TaskItemConvertMeta = { refresh: true }
  view.dispatch(view.state.tr.setMeta(taskItemConvertKey, meta))
}

/** The item a widget at `widgetPos` belongs to. */
function itemAt(doc: ProseNode, widgetPos: number): { pos: number; node: ProseNode } | null {
  const $pos = doc.resolve(widgetPos)
  for (let depth = $pos.depth; depth > 0; depth--) {
    const node = $pos.node(depth)
    if (node.type.name === 'list_item') return { pos: $pos.before(depth), node }
  }
  return null
}

function buildButton(
  handle: TaskItemConvertHandle,
  text: string,
  busy: boolean,
  view: EditorView,
  getPos: () => number | undefined
): HTMLElement {
  const button = document.createElement('button')
  button.type = 'button'
  button.className = 'task-item-convert'
  button.contentEditable = 'false'
  button.textContent = '↗'
  button.title = handle.label
  button.setAttribute('aria-label', `${handle.label}: ${text}`)
  if (busy) button.setAttribute('aria-disabled', 'true')
  button.addEventListener('mousedown', (event) => event.preventDefault())
  button.addEventListener('click', (event) => {
    event.preventDefault()
    if (button.getAttribute('aria-disabled') === 'true') return
    const widgetPos = getPos()
    if (widgetPos == null) return
    const found = itemAt(view.state.doc, widgetPos)
    if (!found) return
    handle.onConvert?.({ pos: found.pos, text: taskItemText(found.node) }, button)
  })
  return button
}

function decorations(
  doc: ProseNode,
  handle: TaskItemConvertHandle,
  marks: BusyMark[]
): DecorationSet {
  if (!handle.label) return DecorationSet.empty
  const busy = new Set(marks.map((mark) => mark.pos))
  const widgets: Decoration[] = []
  doc.descendants((node, pos) => {
    if (!isTaskItem(node)) return true
    const para = node.firstChild
    const text = taskItemText(node)
    if (para?.type.name === 'paragraph' && text && hasOwnText(node)) {
      const end = pos + 2 + para.content.size
      const isBusy = busy.has(pos)
      widgets.push(
        Decoration.widget(end, (view, getPos) => buildButton(handle, text, isBusy, view, getPos), {
          side: 1,
          ignoreSelection: true,
          stopEvent: () => true,
          // A new key rebuilds the button when what it shows changed.
          key: `task-item-convert:${handle.label}:${isBusy}:${text}`,
        })
      )
    }
    return true
  })
  return DecorationSet.create(doc, widgets)
}

/**
 * Moves the marks through a document change. A mark whose item was deleted
 * is dropped: mapped, it would land on whatever took the item's place and
 * disable that item's button.
 */
function mapMarks(tr: Transaction, marks: BusyMark[]): BusyMark[] {
  return marks.flatMap((mark) => {
    const mapped = tr.mapping.mapResult(mark.pos, 1)
    return mapped.deletedAfter ? [] : [{ token: mark.token, pos: mapped.pos }]
  })
}

/** The plugin, bound to a handle the host mutates. */
export const taskItemConvertPlugin = (handle: TaskItemConvertHandle) =>
  $prose(
    () =>
      new Plugin<TaskItemConvertState>({
        key: taskItemConvertKey,
        state: {
          init: (_config, state) => ({ busy: [], decorations: decorations(state.doc, handle, []) }),
          apply: (tr, prev, _old, state) => {
            const meta = tr.getMeta(taskItemConvertKey) as TaskItemConvertMeta | undefined
            // A selection change touches nothing the buttons show.
            if (!tr.docChanged && !meta) return prev
            let busy = tr.docChanged ? mapMarks(tr, prev.busy) : prev.busy
            if (meta && 'mark' in meta) busy = [...busy, meta.mark]
            if (meta && 'clear' in meta) busy = busy.filter((mark) => mark.token !== meta.clear)
            return { busy, decorations: decorations(state.doc, handle, busy) }
          },
        },
        props: {
          decorations: (state) => taskItemConvertKey.getState(state)?.decorations,
        },
      })
  )
