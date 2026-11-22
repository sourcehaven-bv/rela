import {
  computed,
  getCurrentScope,
  onScopeDispose,
  ref,
  shallowRef,
  toValue,
  watch,
  watchEffect,
  type MaybeRefOrGetter,
} from 'vue'
import { usePageHeaderOutlet } from '@/composables/usePageHeader'

/**
 * The browser tab title: `(N) <page> · <app name>` (TKT-FJO08Z4).
 *
 * Three module-level parts, because three places know them. The app shell
 * knows the app name, the routed view knows its page title, and the sidebar
 * owns the inbox count. `useDocumentTitle` in `App.vue` is the one writer of
 * `document.title`; the others only publish their part.
 *
 * The page part is the page header's title when the shell paints one (lists,
 * kanbans and tabbed pages publish it through usePageHeader), otherwise the
 * title the newest live `usePageTitle` call gives. `PageTitle` calls it for
 * every screen that draws one; other views call it with their own heading.
 *
 * A page names itself only once its data is in, so between a navigation and
 * that moment the page part is empty. The tab then keeps the title it has
 * (the previous page's, or the shell's on first load) for up to
 * `TITLE_HOLD_MS`, and only shows the bare app name when no page title came
 * in that time. Without the hold the tab flashed the app name on every
 * navigation.
 */

/** How long the tab keeps its title while the next page has none yet. */
export const TITLE_HOLD_MS = 2000

type TitleSource = () => string | null | undefined

const pageTitles = shallowRef<TitleSource[]>([])
const badgeCount = ref(0)

/**
 * Publishes this screen's page title while the calling scope lives. The
 * newest call wins, so a view mounted after another names the tab.
 */
export function usePageTitle(source: MaybeRefOrGetter<string | null | undefined>) {
  if (!getCurrentScope()) {
    throw new Error('usePageTitle needs a component or effect scope to remove the title again')
  }
  const entry: TitleSource = () => toValue(source)
  pageTitles.value = [...pageTitles.value, entry]
  onScopeDispose(() => {
    pageTitles.value = pageTitles.value.filter((e) => e !== entry)
  })
}

/** Sets the count in front of the title; zero shows none. */
export function setTitleBadgeCount(n: number) {
  badgeCount.value = n
}

/** The title with a `(N) ` prefix, or the title alone for zero. */
export function badgedTitle(base: string, count: number): string {
  return count > 0 ? `(${count}) ${base}` : base
}

/**
 * `<page> · <app>`. Either part alone when the other is empty or when the
 * page is the app name itself, and `rela` when both are empty.
 */
export function composeTitle(page: string, app: string): string {
  const p = page.trim()
  const a = app.trim()
  if (!p || p === a) return a || p || 'rela'
  if (!a) return p
  return `${p} · ${a}`
}

/** Writes `document.title` from the parts. Call once, in the app shell. */
export function useDocumentTitle(appName: MaybeRefOrGetter<string | null | undefined>) {
  const header = usePageHeaderOutlet()
  const page = computed(() => {
    if (header.title.value) return header.title.value
    const titles = pageTitles.value
    for (let i = titles.length - 1; i >= 0; i--) {
      const t = titles[i]()
      if (t) return t
    }
    return ''
  })
  // The title without the badge; null until something may be written, so
  // the shell's own <title> stays during the first hold.
  const base = ref<string | null>(null)
  let hold: ReturnType<typeof setTimeout> | undefined
  const stopHold = () => {
    if (hold !== undefined) clearTimeout(hold)
    hold = undefined
  }
  watch(
    [page, () => toValue(appName) ?? ''],
    ([p, app]) => {
      if (p) {
        stopHold()
        base.value = composeTitle(p, app)
        return
      }
      if (hold !== undefined) return
      hold = setTimeout(() => {
        hold = undefined
        if (!page.value) base.value = composeTitle('', toValue(appName) ?? '')
      }, TITLE_HOLD_MS)
    },
    { immediate: true }
  )
  if (getCurrentScope()) onScopeDispose(stopHold)
  watchEffect(() => {
    if (base.value !== null) document.title = badgedTitle(base.value, badgeCount.value)
  })
}
