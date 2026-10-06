import { onBeforeUnmount, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useUIStore } from '@/stores/ui'
import { useSpaceStore, withSpace } from '@/stores/space'
import { paletteOpen, shortcutsModalOpen } from './useKeyboardShortcuts'

/**
 * The event a native shell dispatches on `window` to drive the SPA. The
 * desktop app's menu bar takes its keyboard shortcuts before the page sees
 * them, so ⌘K or ⌘[ in the menu has to reach the SPA this way rather than as
 * a keydown. In a browser nothing sends it.
 */
export const SHELL_COMMAND_EVENT = 'rela:shell-command'

export interface ShellCommand {
  command: string
  arg?: string
}

/**
 * Runs the commands a native shell sends. Call once, at App level.
 *
 * The shell only names the intent; the SPA decides what it means, so a space
 * is opened at the home the SPA already knows rather than at a path the shell
 * would have to compute. An unknown command is ignored: a newer shell may
 * send one an older page does not know.
 */
export function useShellCommands() {
  const router = useRouter()
  const ui = useUIStore()
  const spaces = useSpaceStore()

  function run({ command, arg }: ShellCommand) {
    switch (command) {
      case 'back':
        router.back()
        break
      case 'forward':
        router.forward()
        break
      case 'palette':
        paletteOpen.value = true
        break
      case 'shortcuts':
        shortcutsModalOpen.value = true
        break
      case 'toggle-sidebar':
        ui.toggleSidebar()
        break
      case 'settings':
        void router.push('/settings')
        break
      case 'space': {
        const s = spaces.spaces.find((sp) => sp.id === arg)
        if (s) void router.push(withSpace(s.home, s.id))
        break
      }
      case 'navigate':
        // Same-origin paths only; the shell builds them from routes it knows.
        if (arg && arg.startsWith('/') && !arg.startsWith('//')) void router.push(arg)
        break
    }
  }

  function onCommand(e: Event) {
    const detail = (e as CustomEvent<ShellCommand>).detail
    if (detail && typeof detail.command === 'string') run(detail)
  }

  onMounted(() => window.addEventListener(SHELL_COMMAND_EVENT, onCommand))
  onBeforeUnmount(() => window.removeEventListener(SHELL_COMMAND_EVENT, onCommand))

  return { run }
}
