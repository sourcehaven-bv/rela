import { watch, type Ref } from 'vue'

/**
 * Keeps keyboard focus on a column's collapse control while it folds and
 * unfolds.
 *
 * The heading's collapse button and the collapsed rail are different
 * elements swapped by `v-if`, so the focused one unmounts on every toggle and
 * focus falls to `<body>`. The board marks the toggled column with `expect`,
 * and once the caller has re-rendered it with the new `collapsed` state, focus
 * moves to the column's other control: the rail after a collapse, the
 * collapse button after an expand.
 *
 * Both controls carry `data-section-id`; the rail is the button itself, the
 * expanded column holds the button inside its heading.
 */
export function useSectionToggleFocus(root: Ref<HTMLElement | undefined>, collapsedState: () => string) {
  let pending: string | undefined

  watch(
    collapsedState,
    () => {
      if (pending === undefined) return
      const holder = root.value?.querySelector<HTMLElement>(`[data-section-id="${CSS.escape(pending)}"]`)
      pending = undefined
      const control = holder?.matches('button')
        ? holder
        : holder?.querySelector<HTMLElement>('.rl-section-heading__collapse')
      control?.focus()
    },
    { flush: 'post' },
  )

  return {
    /** The column `id` is about to fold or unfold at the user's request. */
    expect(id: string) {
      pending = id
    },
  }
}
