<script setup lang="ts">
/**
 * The WYSIWYG markdown editor.
 *
 * Public contract is unchanged from the editor it replaces: `modelValue` in,
 * `update:modelValue` out. `DynamicForm` needs no changes.
 *
 * Three things here are load-bearing:
 *
 * 1. Serialization is pinned by `RELA_STRINGIFY_OPTIONS` and checked against
 *    every entity body in the repository by the corpus test. `guardWriteBack`
 *    then suppresses the reformatting that survives, so opening an entity and
 *    saving it without edits emits nothing.
 * 2. Entity references are `entityRef` nodes that serialize back to the exact
 *    code span they came from. Titles are view-only attributes and never reach
 *    the markdown.
 * 3. Titles come only from the injected `refResolver`, which is fed by the
 *    server's per-principal `mentions` map. The editor never derives a title
 *    from the graph itself.
 */
import { ref, onMounted, onBeforeUnmount, watch, shallowRef, computed, nextTick } from 'vue'
import {
  Editor,
  rootCtx,
  defaultValueCtx,
  remarkStringifyOptionsCtx,
  editorViewOptionsCtx,
} from '@milkdown/kit/core'
import { commonmark, remarkPreserveEmptyLinePlugin } from '@milkdown/kit/preset/commonmark'
import { gfm } from '@milkdown/kit/preset/gfm'
import { history } from '@milkdown/kit/plugin/history'
import { block, BlockProvider } from '@milkdown/kit/plugin/block'
import { cursor } from '@milkdown/kit/plugin/cursor'
import { trailing } from '@milkdown/kit/plugin/trailing'
import { listener, listenerCtx } from '@milkdown/kit/plugin/listener'
import { SlashProvider, slashFactory } from '@milkdown/kit/plugin/slash'
import { replaceAll, getMarkdown, callCommand, $prose } from '@milkdown/kit/utils'
import { editorViewCtx } from '@milkdown/kit/core'
import type { EditorView } from '@milkdown/kit/prose/view'
import { TextSelection, Plugin, PluginKey } from '@milkdown/kit/prose/state'
import type { EditorState } from '@milkdown/kit/prose/state'
import { lift } from '@milkdown/kit/prose/commands'
import '@milkdown/kit/prose/view/style/prosemirror.css'
import '@milkdown/kit/prose/tables/style/tables.css'
import '@milkdown/kit/prose/gapcursor/style/gapcursor.css'
import './milkdownEditor.css'

import { RELA_STRINGIFY_OPTIONS } from './serializerContract'
import { entityRefNode, isValidEntityRefId } from './entityRefNode'
import {
  entityRefResolutionPlugin,
  buildResolutionTransaction,
  isResolutionTransaction,
  type ResolverHandle,
} from './entityRefResolution'
import { guardWriteBack } from './writeBackGuard'
import { parseMentionQuery } from './mentionQuery'
import { useMentionMenu } from './useMentionMenu'
import { INLINE_COMMANDS, BLOCK_COMMANDS, type EditorCommand } from './editorCommands'
import { activeCommandIds } from './activeFormats'
import { unavailableCommandIds } from './commandAvailability'
import {
  TABLE_COMMANDS,
  DIRECT_TABLE_COMMANDS,
  GUARDED_TABLE_COMMANDS,
  canRunTableCommand,
  canAddRowBefore,
  runTableCommand,
  isInTable,
} from './tableCommands'
import MentionMenu from './MentionMenu.vue'
import EditorToolbar from './EditorToolbar.vue'
import EntityPickerModal from '../EntityPickerModal.vue'
import type { EntityRefResolver } from '@/utils/markdown'
import type { Entity } from '@/types'
import { entityDisplayTitle } from '@/utils/entityDisplay'

const props = defineProps<{
  modelValue: string
  placeholder?: string
  /**
   * Resolves an entity ID to its title and type, for display only.
   *
   * Supplied by the form from the server's `mentions` map, which is computed
   * per principal through the read gate. Absent means every reference renders
   * as its bare ID, which is the correct degraded state rather than an error.
   */
  refResolver?: EntityRefResolver
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const editorRoot = ref<HTMLElement | null>(null)
const menuRoot = ref<HTMLElement | null>(null)
const blockHandleRoot = ref<HTMLElement | null>(null)
const editor = shallowRef<Editor | null>(null)

/** True while the document is empty, so the placeholder shows. */
const isEmpty = ref(true)

const menu = useMentionMenu()
const menuState = computed(
  () => menu.state as unknown as import('./useMentionMenu').MentionMenuState
)

const showPlaceholder = computed(() => isEmpty.value)

/**
 * Command ids whose formatting is active at the cursor.
 *
 * Recomputed from a ProseMirror plugin on every state change rather than
 * polled, so the toolbar tracks the selection as it moves.
 */
const activeIds = ref<Set<string>>(new Set())

/**
 * Command ids that would do nothing if pressed, so their buttons disable.
 *
 * Heading inside a list item is the motivating case: the command cannot apply
 * there, and a button that looks pressable but no-ops gives the user nothing
 * to reason about.
 */
const unavailableIds = ref<Set<string>>(new Set())

const inlineCommands = INLINE_COMMANDS
/** The toolbar's block buttons. The `/` menu offers the same set, filtered. */
const toolbarBlockCommands = BLOCK_COMMANDS
/** Every command the toolbar can light up, probed together on each change. */
/** The table group, shown only while the cursor is inside a table. */
const tableCommands = TABLE_COMMANDS
/**
 * Whether to show the table group at all.
 *
 * Hidden rather than disabled: seven permanently-greyed buttons is a lot of
 * dead chrome to carry on every paragraph, and unlike the block commands
 * these have no meaning outside a table to hint at.
 */
const showTableGroup = ref(false)

const ALL_COMMANDS = [...INLINE_COMMANDS, ...BLOCK_COMMANDS, ...TABLE_COMMANDS]

/**
 * The markdown the editor was last loaded with.
 *
 * `guardWriteBack` compares against this to tell a real edit from a
 * round-trip artifact, so it must track every value that comes in from the
 * parent as well as every value we emit.
 */
let loadedValue = props.modelValue
/**
 * Whether the user has changed the document since it was loaded.
 *
 * Set from a ProseMirror plugin rather than from the markdown listener,
 * because not every edit reaches that listener the same way: an insertion
 * dispatched directly on the view (the toolbar picker) changes the document
 * without going through the typing path. Reading `docChanged` catches every
 * route into the document, and it is what `guardWriteBack` needs to tell a
 * real edit from a round-trip artifact. Getting this wrong discards the edit.
 *
 * Resolution transactions are excluded: a title arriving from the server
 * changes node attributes, not content, and must not count as an edit. So is
 * everything before `armed` — see the tracker plugin for why.
 */
let dirty = false

/**
 * Whether document changes now count as user edits.
 *
 * False while a document is being loaded, because loading provokes
 * normalization that is not an edit.
 */
let armed = false

// Held in a mutable box so the ProseMirror plugin, created once at mount,
// always reads the current resolver rather than the one captured at build.
const resolverHandle: ResolverHandle = { resolver: props.refResolver }

/**
 * The commonmark preset with `remarkPreserveEmptyLinePlugin` taken out.
 *
 * That plugin keeps blank lines between paragraphs, but it does so by
 * serializing EVERY empty paragraph as a literal `<br />` — including the
 * empty cells of a table, so adding a row or column wrote raw HTML into the
 * stored markdown.
 *
 * Removing it is a straight improvement rather than a trade: blank runs are
 * preserved exactly as written instead of being normalized, and no `<br />`
 * appears. Filtered here rather than through `Editor.remove`, which returns a
 * promise and would break the builder chain.
 */
const EMPTY_LINE_PLUGIN_PARTS = new Set<unknown>(remarkPreserveEmptyLinePlugin)
const COMMONMARK_WITHOUT_EMPTY_LINE_PLUGIN = commonmark.filter(
  (plugin) => !EMPTY_LINE_PLUGIN_PARTS.has(plugin)
)

const dirtyTrackerKey = new PluginKey('rela-dirty-tracker')

let slashProvider: SlashProvider | null = null
let blockProvider: BlockProvider | null = null
/** Where the active `@` query starts, so insertion replaces trigger and query. */
let activeMatchLength = 0

/** True when the document holds nothing a user has written. */
function isDocEmpty(state: EditorState): boolean {
  const { doc } = state
  if (doc.childCount === 0) return true
  if (doc.childCount > 1) return false
  const first = doc.firstChild
  return first !== null && first.type.name === 'paragraph' && first.content.size === 0
}

/**
 * Recomputes everything the UI derives from editor state.
 *
 * Called from the tracker plugin on every transaction AND directly after a
 * load, because `appendTransaction` does not run for the initial document.
 */
function refreshDerivedState(state: EditorState): void {
  isEmpty.value = isDocEmpty(state)
  const active = activeCommandIds(state, ALL_COMMANDS)
  activeIds.value = active
  showTableGroup.value = isInTable(state)
  const e = editor.value
  if (!e) return

  const unavailable = unavailableCommandIds(
    e.ctx,
    state,
    ALL_COMMANDS.filter(
      (c) => !DIRECT_TABLE_COMMANDS.has(c.id) && !GUARDED_TABLE_COMMANDS.has(c.id)
    ),
    active,
    lift(state)
  )
  // The deletes are not registered slices, so they answer for themselves.
  for (const cmd of ALL_COMMANDS) {
    if (DIRECT_TABLE_COMMANDS.has(cmd.id) && !canRunTableCommand(cmd.id, state)) {
      unavailable.add(cmd.id)
    }
  }
  // AddRowBefore IS a slice, and it claims to apply in the header row before
  // corrupting the table. Its own guard overrides the dry run.
  if (!canAddRowBefore(state)) unavailable.add('addRowBefore')
  unavailableIds.value = unavailable
}

function currentView(): EditorView | null {
  const e = editor.value
  if (!e) return null
  try {
    return e.ctx.get(editorViewCtx)
  } catch {
    return null
  }
}

/**
 * Replaces the `@query` before the cursor with an entityRef node.
 *
 * The title the picker just displayed is carried onto the node so the
 * reference reads correctly straight away, without waiting for a mentions
 * refresh that will not include a just-inserted ID.
 */
function insertRef(item: Entity): void {
  const view = currentView()
  if (!view) return
  if (!isValidEntityRefId(item.id)) return

  const { state } = view
  const { $from } = state.selection
  const to = $from.pos
  // Replaces the trigger and the query together, so the `@abc` the user typed
  // does not survive alongside the node it produced.
  const from = Math.max($from.start(), to - activeMatchLength)

  const node = state.schema.nodes.entityRef.create({
    id: item.id,
    title: entityDisplayTitle(item) || null,
    entityType: item.type ?? null,
    inaccessible: false,
  })
  const tr = state.tr.replaceWith(from, to, node)
  // A space after the reference so the user can keep typing prose without the
  // next character being absorbed into the node.
  tr.insertText(' ', from + 1)
  tr.setSelection(TextSelection.create(tr.doc, from + 2))
  view.dispatch(tr)
  menu.close()
  view.focus()
}

// The toolbar's entity-reference picker, kept from the editor this replaces.
// The `@` menu is the fast path; this stays for discovery, since a user who
// has not learned the trigger still needs a way in.
const pickerOpen = ref(false)

/** Inserts a reference at the cursor, replacing any selection. */
function insertRefAtCursor(id: string, title: string, entityType: string | null): void {
  const view = currentView()
  if (!view) return
  if (!isValidEntityRefId(id)) return
  const { state } = view
  const node = state.schema.nodes.entityRef.create({
    id,
    title: title || null,
    entityType,
    inaccessible: false,
  })
  const { from, to } = state.selection
  const tr = state.tr.replaceWith(from, to, node)
  tr.insertText(' ', from + 1)
  tr.setSelection(TextSelection.create(tr.doc, from + 2))
  view.dispatch(tr)
}

function onPickerSelect(id: string): void {
  // The picker emits an ID only, so the title resolves through the mentions
  // map on the next pass, or falls back to the ID. It cannot be looked up
  // locally without routing around the read gate.
  const hit = resolverHandle.resolver?.(id) ?? null
  insertRefAtCursor(id, hit?.title ?? '', hit?.type ?? null)
}

function onPickerClose(): void {
  pickerOpen.value = false
  void nextTick(() => currentView()?.focus())
}

/**
 * Runs a formatting command and returns focus to the document.
 *
 * A toolbar button steals focus on mousedown unless prevented, and even with
 * that the command must run against a focused view for the selection to be
 * where the user left it. Refocusing after dispatch covers both.
 */
function runCommand(cmd: EditorCommand): void {
  const e = editor.value
  if (!e) return
  if (unavailableIds.value.has(cmd.id)) return

  // Row/column deletes bypass the command manager; see `tableCommands`.
  if (DIRECT_TABLE_COMMANDS.has(cmd.id)) {
    const view = currentView()
    if (view) {
      runTableCommand(cmd.id, view.state, view.dispatch.bind(view))
      view.focus()
    }
    return
  }

  // An active block command runs its inverse, so the button toggles rather
  // than no-opping on a block that is already that type.
  const active = activeIds.value.has(cmd.id)
  if (active && cmd.toggleTo === 'lift') {
    // Unwrapping a blockquote has no preset command; see `toggleTo`.
    const view = currentView()
    if (view) lift(view.state, view.dispatch)
  } else if (active && cmd.toggleTo) {
    e.action(callCommand(cmd.toggleTo))
  } else {
    e.action(callCommand(cmd.command, cmd.payload))
  }
  currentView()?.focus()
}

function onMenuPick(index: number): void {
  menu.setHighlight(index)
  const item = menu.current()
  if (item) insertRef(item as Entity)
}

function onMenuHover(index: number): void {
  menu.setHighlight(index)
}

/**
 * Keyboard handling for whichever menu is open.
 *
 * Bound on the wrapper in the capture phase so it runs before ProseMirror's
 * own keymap: without that, Enter inserts a paragraph break and the arrow keys
 * move the cursor instead of the highlight.
 *
 * The two menus cannot both be open: `@` needs a boundary character before it
 * and `/` only fires at the start of a block, so the triggers are mutually
 * exclusive by construction. The mention menu is still checked first, so a
 * stray overlap would resolve one way rather than acting on both.
 */
function onKeydownCapture(event: KeyboardEvent): void {
  if (!menu.state.open) return
  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      menu.moveHighlight(1)
      break
    case 'ArrowUp':
      event.preventDefault()
      menu.moveHighlight(-1)
      break
    case 'Enter':
    case 'Tab': {
      const item = menu.current()
      if (!item) return
      event.preventDefault()
      insertRef(item as Entity)
      break
    }
    case 'Escape':
      event.preventDefault()
      menu.close()
      break
    default:
      break
  }
}

onMounted(async () => {
  if (!editorRoot.value || !menuRoot.value) return

  const slash = slashFactory('rela-entity-mention')

  // Marks the editor dirty on a document change the USER caused.
  //
  // `docChanged` alone is not that signal. Loading a document runs
  // normalization of its own: with a heading and a table in the same body,
  // ProseMirror's table plugin rewrites the column widths before the user has
  // touched anything. Counting that as an edit told the write-back guard the
  // body had been edited, so it wrote the reformatted table back and put a
  // diff in git for an entity that was only opened.
  //
  // `armed` is what separates the two. It is set once loading has settled, so
  // load-time normalization is ignored and everything after it is real. A
  // reload re-disarms for the same reason.
  const dirtyTracker = $prose(
    () =>
      new Plugin({
        key: dirtyTrackerKey,
        appendTransaction: (transactions, _oldState, newState) => {
          for (const tr of transactions) {
            if (armed && tr.docChanged && !isResolutionTransaction(tr)) dirty = true
          }
          // Recomputed here rather than in a watcher so the toolbar updates
          // on selection moves too, which change state without changing the
          // document.
          refreshDerivedState(newState)
          return null
        },
      })
  )

  editor.value = await Editor.make()
    .config((ctx) => {
      ctx.set(rootCtx, editorRoot.value as HTMLElement)
      ctx.set(defaultValueCtx, props.modelValue)
      // `md-body` is the marker class for the app's shared markdown stylesheet
      // (styles/markdown-content.css), the documented single source of truth
      // for how a rendered body looks. Wearing it means the editing surface
      // inherits the exact headings, lists, tables, quotes and code styling
      // the entity view will show, so WYSIWYG is true by construction instead
      // of by a parallel set of rules that would drift.
      ctx.update(editorViewOptionsCtx, (prev) => ({
        ...prev,
        attributes: { ...(prev.attributes ?? {}), class: 'milkdown-prose md-body' },
      }))
      // Merge rather than replace: the defaults carry Milkdown's own remark
      // handlers, and dropping them would break serialization of every node
      // type the presets contribute.
      ctx.update(remarkStringifyOptionsCtx, (prev) => ({
        ...prev,
        ...RELA_STRINGIFY_OPTIONS,
      }))

      ctx.get(listenerCtx).markdownUpdated((_ctx, markdown) => {
        if (markdown === loadedValue) return
        emit('update:modelValue', markdown)
      })

      ctx.set(slash.key, {
        view: () => {
          slashProvider = new SlashProvider({
            content: menuRoot.value as HTMLElement,
            trigger: '@',
            debounce: 0,
            // The default shouldShow only fires while `@` is the last
            // character typed, so it cannot follow a query. This reads the
            // text before the cursor on every update instead.
            shouldShow: (view) => {
              const match = parseMentionQuery(slashProvider?.getContent(view))
              if (!match) {
                if (menu.state.open) menu.close()
                return false
              }
              activeMatchLength = match.matchLength
              menu.setQuery(match.query)
              return true
            },
          })
          return {
            update: (view, prevState) => slashProvider?.update(view, prevState),
            destroy: () => slashProvider?.destroy(),
          }
        },
      })
    })
    .use(COMMONMARK_WITHOUT_EMPTY_LINE_PLUGIN)
    .use(gfm)
    .use(history)
    .use(listener)
    .use(entityRefNode)
    .use(entityRefResolutionPlugin(resolverHandle))
    .use(dirtyTracker)
    .use(slash)
    // Drag/insert handle in the gutter, plus the drop cursor and gap cursor it
    // needs to be usable: without `cursor` a drag has no visible target, and
    // without `trailing` there is no way to click below a table or code block
    // that ends the document to start a new paragraph.
    .use(block)
    .use(cursor)
    .use(trailing)
    .create()

  // The gutter handle. Built after creation because it needs the editor's ctx.
  // `getOffset` pushes it into the left gutter the shell reserves, so it sits
  // beside the block rather than on top of the first characters.
  if (blockHandleRoot.value && editor.value) {
    blockProvider = new BlockProvider({
      ctx: editor.value.ctx,
      content: blockHandleRoot.value,
      getOffset: () => ({ mainAxis: 8 }),
      getPlacement: () => 'left-start',
    })
  }

  applyResolver()
  const mountedView = currentView()
  // Seed the derived state from the loaded document. `appendTransaction` only
  // runs once a transaction is dispatched, so without this the toolbar shows
  // nothing active until the first edit — a document opened straight onto a
  // heading or a quote had the wrong buttons lit.
  if (mountedView) refreshDerivedState(mountedView.state)
  // Loading is done; from here a document change is the user's doing. Take the
  // settled markdown as the baseline, so the guard compares against what the
  // editor actually holds rather than the bytes it was handed.
  loadedValue = serialize()
  armed = true

  // E2E hook: expose the serialized markdown on the editor root.
  //
  // The markdown is a serialization of the ProseMirror document, not a buffer
  // hanging off a DOM node, so there is nothing for a test to read the way it
  // could read CodeMirror's `getValue`. Compiled out of production builds by
  // the `__E2E_TEST_HOOKS__` define, so neither the hook nor its name ships
  // (issue #890).
  if (__E2E_TEST_HOOKS__ && editorRoot.value) {
    ;(editorRoot.value as HTMLElement & { __relaGetMarkdown?: () => string }).__relaGetMarkdown =
      () => serialize()
  }
})

/** Re-runs resolution over the whole document against the current resolver. */
function applyResolver(): void {
  const view = currentView()
  if (!view) return
  const tr = buildResolutionTransaction(view.state, resolverHandle.resolver)
  if (tr) view.dispatch(tr)
}

watch(
  () => props.refResolver,
  (next) => {
    resolverHandle.resolver = next
    applyResolver()
  }
)

watch(
  () => props.modelValue,
  (next) => {
    const e = editor.value
    if (!e) return
    // Only reload when the parent's value genuinely diverges from what the
    // editor holds. Without the guard, echoing back our own emitted value
    // would rebuild the document and drop the cursor on every keystroke.
    const view = currentView()
    if (!view) return
    const currentMarkdown = serialize()
    if (currentMarkdown === next) return
    loadedValue = next
    dirty = false
    armed = false
    e.action(replaceAll(next))
    applyResolver()
    // Same reason as at mount: a reload replaces the document without the
    // tracker having seen a transaction for the new content.
    const reloaded = currentView()
    if (reloaded) refreshDerivedState(reloaded.state)
    // Same reasoning as at mount: re-baseline on the settled document and only
    // then treat changes as edits.
    loadedValue = serialize()
    armed = true
  }
)

/**
 * The markdown the editor currently holds.
 *
 * Falls back to the last loaded value if the editor is not ready, so a caller
 * never sees an empty body for an entity that has content.
 */
function serialize(): string {
  const e = editor.value
  if (!e) return loadedValue
  try {
    return e.action(getMarkdown())
  } catch {
    return loadedValue
  }
}

defineExpose({
  /**
   * The markdown to save, with round-trip churn suppressed.
   *
   * A caller that saves `modelValue` directly gets the churn; this is the
   * guarded value. Exposed so the form can prefer it.
   */
  guardedValue: () => guardWriteBack(loadedValue, serialize(), dirty),
  /**
   * The live ProseMirror view, for tests only.
   *
   * Two things can only be checked against the real editor. The active-state
   * probes name marks and nodes by string and a miss returns false rather
   * than throwing, so a rename upstream would quietly stop the toolbar
   * lighting up; and a command's slice name is resolved at call time, so a
   * wrong one throws only when the button is pressed. A hand-built schema
   * cannot catch either, since it would be renamed alongside the code.
   *
   * Tests also need it to place a real selection, which is the case a
   * cursor-only test does not cover.
   */
  get editorViewForTest() {
    return currentView()
  },
  get editorInstanceForTest() {
    return editor.value
  },
})

onBeforeUnmount(() => {
  // Close the picker before tearing the editor down so a late `select` cannot
  // fire against a destroyed view.
  pickerOpen.value = false
  menu.dispose()
  slashProvider?.destroy()
  slashProvider = null
  blockProvider?.destroy()
  blockProvider = null
  void editor.value?.destroy()
  editor.value = null
})
</script>

<template>
  <div class="milkdown-editor-shell">
    <EditorToolbar
      :inline-commands="inlineCommands"
      :block-commands="toolbarBlockCommands"
      :active-ids="activeIds"
      :unavailable-ids="unavailableIds"
      :table-commands="tableCommands"
      :show-table-group="showTableGroup"
      @run="runCommand"
      @open-entity-picker="pickerOpen = true"
    />

    <div class="milkdown-editor-body">
      <div ref="editorRoot" class="milkdown-editor" @keydown.capture="onKeydownCapture">
        <!-- Placeholder for an empty document. A sibling element rather than a
             `::before` fed by `attr()`: the text would have to reach the
             pseudo-element through an inline `style`, and the sandboxed-app CSP
             this editor is headed for (`style-src` with no `'unsafe-inline'`)
             blocks the `style` attribute outright. A real element needs no
             inline style, so Phase 4 does not have to undo this.

             `aria-hidden` because ProseMirror's own textbox role already names
             the field; a screen reader announcing both would read it twice. -->
        <div v-if="showPlaceholder" class="milkdown-placeholder" aria-hidden="true">
          {{ props.placeholder || 'Markdown content...' }}
        </div>
      </div>
    </div>

    <!-- The gutter handle BlockProvider positions against the hovered block.
         Purely a drag affordance: the grip is what ProseMirror's block service
         binds its drag events to. -->
    <div ref="blockHandleRoot" class="block-handle" data-show="false">
      <span class="block-handle-grip" aria-hidden="true">
        <svg viewBox="0 0 10 16" width="10" height="16" fill="currentColor">
          <circle cx="2.5" cy="3" r="1.3" />
          <circle cx="7.5" cy="3" r="1.3" />
          <circle cx="2.5" cy="8" r="1.3" />
          <circle cx="7.5" cy="8" r="1.3" />
          <circle cx="2.5" cy="13" r="1.3" />
          <circle cx="7.5" cy="13" r="1.3" />
        </svg>
      </span>
    </div>

    <!-- Outside the editable div on purpose. SlashProvider positions THIS
         element (it is what `content` is set to) and appends it to the
         editor's parent; leaving it inside the contenteditable would make the
         menu part of the document the user is editing. -->
    <div ref="menuRoot" class="mention-menu-anchor" data-show="false">
      <MentionMenu
        :state="menuState"
        :min-query-length="menu.minQueryLength"
        @pick="onMenuPick"
        @hover="onMenuHover"
      />
    </div>

    <EntityPickerModal :open="pickerOpen" @select="onPickerSelect" @close="onPickerClose" />
  </div>
</template>
