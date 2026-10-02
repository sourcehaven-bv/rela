<script setup lang="ts">
/**
 * A WYSIWYG markdown editor.
 *
 * `modelValue` in, `update:modelValue` out: the value is markdown text at both
 * ends, so the component can be dropped in where a textarea was.
 *
 * Presentational, in the sense the rest of the library is: it holds no
 * fetching, no permissions and no schema. What it does own is the markdown
 * round trip, which is not presentation — parse, edit, serialize — and the
 * toolbar state derived from the ProseMirror selection.
 *
 * ## Extending it
 *
 * The generic command set stops at commonmark plus GFM. An app that needs its
 * own nodes (an entity reference, a mention, an embed) supplies them three
 * ways, and all three are needed together:
 *
 * - `plugins` adds Milkdown plugins to the `.use()` chain, so app nodes join
 *   the same schema the editor builds. This is why `@milkdown/kit` is a peer
 *   dependency: a second copy would give the app a different `Schema` class
 *   and its nodes would be rejected by this editor's document.
 * - `#overlays` renders app UI (a mention menu, say) against the live view,
 *   inside the positioned anchor a Milkdown `SlashProvider` expects.
 * - `#toolbar-extra` passes buttons through to the toolbar.
 *
 * ## What it deliberately does not do
 *
 * There is no write-back guard here. Whether re-serialized markdown may be
 * saved over the original depends on what the original is worth — a git-backed
 * corpus and a draft in local state want opposite answers — so the editor
 * reports what it holds and the app decides. `serializerContract.ts` exports
 * `isSemanticallyEqual` for apps that need to make that call.
 */
import { ref, shallowRef, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import {
  Editor,
  rootCtx,
  defaultValueCtx,
  editorViewCtx,
  editorViewOptionsCtx,
} from '@milkdown/kit/core'
import { gfm } from '@milkdown/kit/preset/gfm'
import { history } from '@milkdown/kit/plugin/history'
import { block, BlockProvider } from '@milkdown/kit/plugin/block'
import { cursor } from '@milkdown/kit/plugin/cursor'
import { trailing } from '@milkdown/kit/plugin/trailing'
import { listener, listenerCtx } from '@milkdown/kit/plugin/listener'
import { replaceAll, getMarkdown, callCommand, $prose } from '@milkdown/kit/utils'
import type { EditorView } from '@milkdown/kit/prose/view'
import { Plugin, PluginKey } from '@milkdown/kit/prose/state'
import type { EditorState } from '@milkdown/kit/prose/state'
import { lift } from '@milkdown/kit/prose/commands'
import type { Options as RemarkStringifyOptions } from 'remark-stringify'
import '@milkdown/kit/prose/view/style/prosemirror.css'
import '@milkdown/kit/prose/tables/style/tables.css'
import '@milkdown/kit/prose/gapcursor/style/gapcursor.css'
import './rlMarkdownEditor.css'

import { MARKDOWN_COMMONMARK, configureSerializer } from './editorPreset'
import { taskList } from './taskListItem'
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
import RlEditorToolbar from './RlEditorToolbar.vue'

const props = withDefaults(
  defineProps<{
    /** The document, as markdown. */
    modelValue?: string
    placeholder?: string
    /** Hides the toolbar, for a surface that supplies its own or none. */
    hideToolbar?: boolean
    /**
     * Extra Milkdown plugins, added after the built-in presets.
     *
     * Ordering matters to Milkdown: a plugin that extends a node must load
     * after the preset contributing it. These load last for that reason.
     */
    plugins?: readonly unknown[]
    /**
     * Serializer options, merged over the library defaults.
     *
     * Set these when the app's stored markdown follows other conventions;
     * see `serializerContract.ts`.
     */
    stringifyOptions?: RemarkStringifyOptions
    /**
     * Extra classes for the editable element.
     *
     * The hook for an app that already has a stylesheet for rendered
     * markdown: naming its marker class here makes the editing surface
     * inherit it, so WYSIWYG holds by construction rather than by two sets of
     * prose rules kept in step by hand.
     */
    contentClass?: string
    /**
     * Renders the document without allowing edits, for showing markdown where
     * a plain text dump would lose its structure. The toolbar is hidden with
     * it, since none of its commands can run.
     */
    readonly?: boolean
  }>(),
  {
    modelValue: '',
    placeholder: 'Write something...',
    hideToolbar: false,
    plugins: () => [],
    stringifyOptions: undefined,
    contentClass: '',
    readonly: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** The editor is mounted and its view is usable. */
  ready: [view: EditorView]
}>()

const editorRoot = ref<HTMLElement | null>(null)
const overlayRoot = ref<HTMLElement | null>(null)
const blockHandleRoot = ref<HTMLElement | null>(null)
const editor = shallowRef<Editor | null>(null)
const view = shallowRef<EditorView | null>(null)

/** True while the document is empty, so the placeholder shows. */
const isEmpty = ref(true)

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

/**
 * Whether to show the table group at all.
 *
 * Hidden rather than disabled: seven permanently-greyed buttons is a lot of
 * dead chrome to carry on every paragraph, and unlike the block commands
 * these have no meaning outside a table to hint at.
 */
const showTableGroup = ref(false)

const showPlaceholder = computed(() => isEmpty.value)

/** Every command the toolbar can light up, probed together on each change. */
const ALL_COMMANDS = [...INLINE_COMMANDS, ...BLOCK_COMMANDS, ...TABLE_COMMANDS]

/**
 * The markdown the parent currently believes the document holds.
 *
 * Starts as the value it handed us and moves forward on every emit. Both jobs
 * it does are needed, and conflating them is a lost edit:
 *
 * - It suppresses the echo of our own load. Parsing and re-serializing
 *   normalizes, and Milkdown's listener reports that as an update, so opening
 *   a document would otherwise emit one immediately.
 * - It suppresses re-emitting a value the parent already has, which would
 *   bounce back through the `modelValue` watcher and rebuild the document.
 *
 * It must NOT stay pinned to the loaded value. Doing that makes an edit and
 * its reversal emit nothing on the way back: the markdown matches what was
 * loaded, the emit is skipped as "nothing to say", and the parent keeps the
 * intermediate value it was last told about.
 */
let settledValue = props.modelValue

/** False while a document is loading, when changes are not user edits. */
let armed = false

let blockProvider: BlockProvider | null = null
const dirtyTrackerKey = new PluginKey('rl-editor-tracker')

/** True when the document holds nothing a user has written. */
function isDocEmpty(state: EditorState): boolean {
  const { doc } = state
  if (doc.childCount === 0) return true
  if (doc.childCount > 1) return false
  const first = doc.firstChild
  return first !== null && first.type.name === 'paragraph' && first.content.size === 0
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
 * Recomputes everything the toolbar derives from editor state.
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
      (c) => !DIRECT_TABLE_COMMANDS.has(c.id) && !GUARDED_TABLE_COMMANDS.has(c.id),
    ),
    active,
    lift(state),
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

/**
 * The markdown the editor currently holds.
 *
 * Falls back to the last loaded value if the editor is not ready, so a caller
 * never sees an empty document for content that exists.
 */
function serialize(): string {
  const e = editor.value
  if (!e) return settledValue
  try {
    return e.action(getMarkdown())
  } catch {
    return settledValue
  }
}

function emitIfChanged(markdown: string): void {
  if (!armed) return
  if (markdown === settledValue) return
  // Moved BEFORE the emit, not after: the parent may write the value straight
  // back, which runs the `modelValue` watcher synchronously, and that watcher
  // compares against this.
  settledValue = markdown
  emit('update:modelValue', markdown)
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
    const v = currentView()
    if (v) {
      runTableCommand(cmd.id, v.state, v.dispatch.bind(v))
      v.focus()
    }
    return
  }

  // An active block command runs its inverse, so the button toggles rather
  // than no-opping on a block that is already that type.
  const active = activeIds.value.has(cmd.id)
  if (active && cmd.toggleTo === 'lift') {
    // Unwrapping a blockquote has no preset command; see `toggleTo`.
    const v = currentView()
    if (v) lift(v.state, v.dispatch)
  } else if (active && cmd.toggleTo) {
    e.action(callCommand(cmd.toggleTo))
  } else {
    e.action(callCommand(cmd.command, cmd.payload))
  }
  currentView()?.focus()
}

onMounted(async () => {
  if (!editorRoot.value) return

  // Recomputes the toolbar on every transaction, including selection moves,
  // which change state without changing the document.
  const tracker = $prose(
    () =>
      new Plugin({
        key: dirtyTrackerKey,
        appendTransaction: (_transactions, _oldState, newState) => {
          refreshDerivedState(newState)
          return null
        },
      }),
  )

  editor.value = await Editor.make()
    .config((ctx) => {
      ctx.set(rootCtx, editorRoot.value as HTMLElement)
      ctx.set(defaultValueCtx, props.modelValue)
      ctx.update(editorViewOptionsCtx, (prev) => ({
        ...prev,
        // ProseMirror's own gate, so the document still renders and stays
        // selectable while every edit transaction is refused.
        editable: () => !props.readonly,
        attributes: {
          ...(prev.attributes ?? {}),
          class: [
            'rl-markdown-editor__content',
            props.readonly ? 'rl-markdown-editor__content--readonly' : '',
            props.contentClass,
          ]
            .filter(Boolean)
            .join(' '),
        },
      }))
      configureSerializer(ctx, props.stringifyOptions)

      ctx.get(listenerCtx).markdownUpdated((_ctx, markdown) => {
        emitIfChanged(markdown)
      })
    })
    .use(MARKDOWN_COMMONMARK)
    .use(gfm)
    .use(history)
    .use(listener)
    // After `gfm`: it contributes the `checked` attribute this renders.
    .use(taskList)
    .use(tracker)
    // Drag/insert handle in the gutter, plus the drop cursor and gap cursor it
    // needs to be usable: without `cursor` a drag has no visible target, and
    // without `trailing` there is no way to click below a table or code block
    // that ends the document to start a new paragraph.
    .use(block)
    .use(cursor)
    .use(trailing)
    // Last, so an app plugin can extend anything the presets contribute.
    .use(props.plugins as Parameters<Editor['use']>[0])
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

  const mounted = currentView()
  view.value = mounted
  // Seed the derived state from the loaded document. `appendTransaction` only
  // runs once a transaction is dispatched, so without this the toolbar shows
  // nothing active until the first edit — a document opened straight onto a
  // heading or a quote would have the wrong buttons lit.
  if (mounted) refreshDerivedState(mounted.state)
  settledValue = serialize()
  armed = true
  if (mounted) emit('ready', mounted)
})

watch(
  () => props.modelValue,
  (next) => {
    const e = editor.value
    if (!e) return
    // Only reload when the parent's value genuinely diverges from what the
    // editor holds. Without the guard, echoing back our own emitted value
    // would rebuild the document and drop the cursor on every keystroke.
    if (serialize() === next) return
    armed = false
    e.action(replaceAll(next))
    void nextTick(() => {
      const reloaded = currentView()
      view.value = reloaded
      // Same reason as at mount: a reload replaces the document without the
      // tracker having seen a transaction for the new content.
      if (reloaded) refreshDerivedState(reloaded.state)
      settledValue = serialize()
      armed = true
    })
  },
)

defineExpose({
  /**
   * Pushes any pending change to the parent immediately.
   *
   * Milkdown's markdown listener is debounced, so a change made and saved
   * within that window never reaches the parent. A form must call this before
   * reading its content model — and an app plugin that inserts through a
   * direct view dispatch has the same problem.
   */
  flush: (): void => {
    emitIfChanged(serialize())
  },
  /** The markdown the editor currently holds. */
  getMarkdown: (): string => serialize(),
  /** The live ProseMirror view, for app plugins and tests. */
  get view(): EditorView | null {
    return currentView()
  },
  /** The Milkdown instance, for app code that needs its ctx. */
  get editor(): Editor | null {
    return editor.value
  },
})

onBeforeUnmount(() => {
  blockProvider?.destroy()
  blockProvider = null
  void editor.value?.destroy()
  editor.value = null
  view.value = null
})
</script>

<template>
  <div class="rl-markdown-editor" :class="{ 'rl-markdown-editor--readonly': props.readonly }">
    <RlEditorToolbar
      v-if="!props.hideToolbar && !props.readonly"
      :inline-commands="INLINE_COMMANDS"
      :block-commands="BLOCK_COMMANDS"
      :active-ids="activeIds"
      :unavailable-ids="unavailableIds"
      :table-commands="TABLE_COMMANDS"
      :show-table-group="showTableGroup"
      @run="runCommand"
    >
      <template v-if="$slots['toolbar-extra']" #extra="slotProps">
        <slot name="toolbar-extra" v-bind="slotProps" :view="view" />
      </template>
    </RlEditorToolbar>

    <div class="rl-markdown-editor__body">
      <div ref="editorRoot">
        <!-- A sibling element rather than a `::before` fed by `attr()`: the
             text would have to reach the pseudo-element through an inline
             `style`, which a strict `style-src` policy blocks outright.

             `aria-hidden` because ProseMirror's own textbox role already names
             the field; a screen reader announcing both would read it twice. -->
        <div v-if="showPlaceholder" class="rl-markdown-editor__placeholder" aria-hidden="true">
          {{ props.placeholder }}
        </div>
      </div>
    </div>

    <!-- The gutter handle BlockProvider positions against the hovered block.
         Purely a drag affordance: the grip is what ProseMirror's block service
         binds its drag events to. -->
    <div ref="blockHandleRoot" class="rl-markdown-editor__block-handle" data-show="false">
      <span class="rl-markdown-editor__grip" aria-hidden="true">
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

    <!-- Outside the editable element on purpose: anything inside it is part of
         the document the user is editing. This is what an app's SlashProvider
         should be given as its `content`, which is why the element exists even
         when the slot is empty. -->
    <div ref="overlayRoot" class="rl-markdown-editor__overlay-anchor" data-show="false">
      <slot name="overlays" :view="view" :anchor="overlayRoot" />
    </div>
  </div>
</template>
