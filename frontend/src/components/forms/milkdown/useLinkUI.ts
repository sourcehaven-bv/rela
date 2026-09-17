/**
 * State and operations for the link dialog and the link panel.
 *
 * Lives outside `MilkdownEditor.vue` for the same reason `useMentionMenu` does:
 * the component is already far past the 500-line god-component threshold, and
 * a feature with two floating surfaces plus a dialog would push it well past a
 * thousand. The component keeps the wiring; the decisions live here.
 *
 * Everything that WRITES goes through one of the three operations at the
 * bottom. Everything that READS is derived from a ProseMirror state the caller
 * passes in — this module never reaches for the view itself, so it cannot
 * accidentally dispatch during a read and arm the write-back guard.
 */
import { ref, computed, type Ref } from 'vue'
import type { EditorView } from '@milkdown/kit/prose/view'
import type { EditorState } from '@milkdown/kit/prose/state'
import { TextSelection } from '@milkdown/kit/prose/state'
import { findLinkAt, type LinkAtSelection } from './linkSelection'

export interface LinkDialogState {
  open: boolean
  initialUrl: string
  initialText: string
  needsText: boolean
  editing: boolean
}

export interface LinkSubmitPayload {
  url: string
  text: string
  strippedParams: boolean
}

export interface UseLinkUI {
  dialog: Ref<LinkDialogState>
  /** The link the panel is describing, or null when it should be hidden. */
  panelLink: Ref<LinkAtSelection | null>
  /** True while the caret sits in a link, which reveals the unlink button. */
  inLink: Ref<boolean>
  /** Recomputed on every state change; decides the panel and the toolbar. */
  refresh: (state: EditorState) => void
  openForSelection: (view: EditorView) => void
  openForPanel: () => void
  closeDialog: () => void
  submit: (view: EditorView, payload: LinkSubmitPayload) => void
  unlink: (view: EditorView) => void
  /** Suppresses the panel for the link at this position until the caret leaves. */
  dismiss: () => void
  /** True when a panel is showing and Escape should close it rather than bubble. */
  panelOpen: Ref<boolean>
}

export function useLinkUI(): UseLinkUI {
  const dialog = ref<LinkDialogState>({
    open: false,
    initialUrl: '',
    initialText: '',
    needsText: false,
    editing: false,
  })

  const currentLink = ref<LinkAtSelection | null>(null)

  /**
   * The link the user dismissed with Escape, remembered by its range.
   *
   * Without this, Escape does nothing visible. The panel's visibility is
   * derived from the selection, so closing it just means the next state change
   * recomputes the same answer and shows it again — the caret is still in the
   * link. The `@` menu hit this exact problem and solved it the same way, with
   * `dismissedQuery`.
   *
   * Cleared as soon as the caret is somewhere else, so the panel is one
   * Escape away from being useful again rather than gone for the session.
   */
  const dismissedAt = ref<{ from: number; to: number } | null>(null)

  const panelLink = computed(() => {
    const link = currentLink.value
    if (!link) return null
    const d = dismissedAt.value
    if (d && d.from === link.from && d.to === link.to) return null
    return link
  })

  const inLink = computed(() => currentLink.value !== null)
  const panelOpen = computed(() => panelLink.value !== null)

  function refresh(state: EditorState): void {
    const link = findLinkAt(state)
    currentLink.value = link
    // The dismissal is tied to one link. Moving off it — to another link, or
    // to no link — makes the memory stale, so drop it.
    const d = dismissedAt.value
    if (d && (!link || d.from !== link.from || d.to !== link.to)) dismissedAt.value = null
  }

  function dismiss(): void {
    const link = currentLink.value
    if (link) dismissedAt.value = { from: link.from, to: link.to }
  }

  function closeDialog(): void {
    dialog.value = { ...dialog.value, open: false }
  }

  /**
   * Opens the dialog for whatever the selection means.
   *
   * Three cases, and the first is the one that matters: a selection touching
   * an existing link is an EDIT, never an insert. Routing it to insert would
   * reach `toggleMark`, which removes a link it partially overlaps and throws
   * away the URL the user is about to type.
   */
  function openForSelection(view: EditorView): void {
    const { state } = view
    const existing = findLinkAt(state)

    if (existing) {
      dialog.value = {
        open: true,
        initialUrl: existing.href,
        initialText: existing.text,
        needsText: false,
        editing: true,
      }
      return
    }

    const { from, to, empty } = state.selection
    dialog.value = {
      open: true,
      initialUrl: '',
      // A collapsed caret has nothing to wrap, so the dialog collects the text
      // as well and inserts it.
      initialText: empty ? '' : state.doc.textBetween(from, to),
      needsText: empty,
      editing: false,
    }
  }

  /** Opens the dialog from the panel's Edit action, prefilled. */
  function openForPanel(): void {
    const link = currentLink.value
    if (!link) return
    dialog.value = {
      open: true,
      initialUrl: link.href,
      initialText: link.text,
      needsText: false,
      editing: true,
    }
  }

  function submit(view: EditorView, payload: LinkSubmitPayload): void {
    const { state } = view
    const linkType = state.schema.marks.link
    if (!linkType) return

    const mark = linkType.create({ href: payload.url })
    const existing = findLinkAt(state)

    if (existing) {
      // Retarget across the link's FULL extent, not the selection. Half a
      // retargeted link is not something a user can have meant, and it would
      // split one link into two with different targets.
      const tr = state.tr
        .removeMark(existing.from, existing.to, linkType)
        .addMark(existing.from, existing.to, mark)
      view.dispatch(tr)
      view.focus()
      return
    }

    const { from, to, empty } = state.selection
    if (empty) {
      // Nothing to wrap: insert the text the dialog collected, carrying the
      // mark, then put the caret after it so typing continues outside the
      // link rather than extending it.
      const text = payload.text || payload.url
      const tr = state.tr.insertText(text, from, to)
      tr.addMark(from, from + text.length, mark)
      tr.setSelection(TextSelection.create(tr.doc, from + text.length))
      view.dispatch(tr)
    } else {
      view.dispatch(state.tr.addMark(from, to, mark))
    }
    view.focus()
  }

  /**
   * Removes the link, keeping its text.
   *
   * Operates on the mark's own range rather than the selection, so it works
   * from a collapsed caret — which is what makes unlink reachable without
   * dragging a selection across the link first.
   */
  function unlink(view: EditorView): void {
    const { state } = view
    const linkType = state.schema.marks.link
    const link = findLinkAt(state)
    if (!linkType || !link) return
    view.dispatch(state.tr.removeMark(link.from, link.to, linkType))
    view.focus()
  }

  return {
    dialog,
    panelLink,
    inLink,
    refresh,
    openForSelection,
    openForPanel,
    closeDialog,
    submit,
    unlink,
    dismiss,
    panelOpen,
  }
}
