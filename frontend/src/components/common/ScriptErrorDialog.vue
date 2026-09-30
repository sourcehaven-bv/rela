<script setup lang="ts">
/**
 * The single script-error dialog, mounted once in App.vue and driven by the
 * store rather than by a parent's `open` prop.
 *
 * Built on `RlModal`, which owns the scrim, the Tab trap, the scroll lock and
 * the shared overlay stack. Two things stay rela's responsibility:
 *
 *  - **Focus restore.** `RlModal` restores focus itself, but to whatever held
 *    it when the dialog opened. That is the wrong element here: a script error
 *    is usually raised from a list action whose row is optimistically removed
 *    before the dialog appears, so the captured element is already detached and
 *    `.focus()` on it silently drops focus to `<body>`. The store captures the
 *    real trigger and checks `document.contains` before restoring, so the
 *    dialog is left to close and the store does the restoring.
 *
 *  - **`useModalStack`.** rela's stack is a different registry from the
 *    library's: `isAnyModalOpen()` gates GLOBAL KEYBOARD SHORTCUTS, and a
 *    dialog that registers only with the library's stack is invisible to it,
 *    so shortcuts keep firing underneath. Both must be called.
 */
import { computed } from 'vue'

import { useScriptErrorStore } from '../../stores/scriptError'
import { useModalStack } from '@/composables/modalStack'
import RlModal from 'rela-components/components/overlay/RlModal.vue'

import ScriptErrorPanel from './ScriptErrorPanel.vue'

const store = useScriptErrorStore()

const open = computed(() => store.current !== null)

useModalStack(open)
</script>

<template>
  <RlModal
    :open="open"
    title="Script error"
    role="alertdialog"
    size="lg"
    @close="store.dismiss()"
  >
    <ScriptErrorPanel v-if="store.current" :error="store.current" />
  </RlModal>
</template>
