/**
 * The editor's side of task-item conversions (TKT-3IBLIP): the plugin bound
 * to a handle that follows the label prop, and the methods the editor
 * exposes to the host. See `taskItemConvert.ts` for what each one does.
 */
import { watch } from 'vue'
import type { EditorView } from '@milkdown/kit/prose/view'
import {
  applyTaskItemResult,
  clearTaskItemBusy,
  markTaskItemBusy,
  refreshTaskItemButtons,
  taskItemConvertPlugin,
  type TaskItemConvertHandle,
  type TaskItemTarget,
} from './taskItemConvert'
import type { EntityRefInsert } from './insertEntityRef'

export function useTaskItemConvert(options: {
  label: () => string | undefined
  onConvert: (target: TaskItemTarget, anchor: HTMLElement) => void
  currentView: () => EditorView | null
  /** Pushes an applied result to the parent at once. */
  flush: () => void
}) {
  const { currentView } = options
  const handle: TaskItemConvertHandle = {
    label: options.label() ?? '',
    onConvert: options.onConvert,
  }
  watch(options.label, (next) => {
    handle.label = next ?? ''
    const view = currentView()
    if (view) refreshTaskItemButtons(view)
  })

  const exposed = {
    /**
     * Applies a finished conversion to the item the busy mark `token` names,
     * and pushes the change to the parent at once, so a save straight after
     * it includes the edit. Returns false when the item is gone or no longer
     * reads `text`.
     */
    applyTaskItemResult(
      token: number,
      text: string,
      mode: 'link' | 'remove',
      ref: EntityRefInsert
    ) {
      const view = currentView()
      if (!view || !applyTaskItemResult(view, token, text, mode, ref)) return false
      options.flush()
      return true
    },
    /**
     * Disables an item's convert button while its request runs. Returns the
     * token for `clearTaskItemBusy`, or null when the item is not found.
     */
    markTaskItemBusy(target: TaskItemTarget): number | null {
      const view = currentView()
      return view ? markTaskItemBusy(view, target) : null
    },
    /** Ends a mark `markTaskItemBusy` made. */
    clearTaskItemBusy(token: number) {
      const view = currentView()
      if (view) clearTaskItemBusy(view, token)
    },
  }
  return { plugin: taskItemConvertPlugin(handle), exposed }
}
