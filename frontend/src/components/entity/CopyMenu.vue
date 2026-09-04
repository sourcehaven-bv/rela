<script setup lang="ts">
/**
 * Renders the copy affordances (RULING 9) for the face on screen: the promote
 * button on a draft policy, the language menu on an English blog post.
 *
 * ## Only ALLOWED offers render, and they render as ABSENT when not
 *
 * A denied offer is not shown disabled — it is not shown at all. A disabled
 * control advertises a capability while refusing it, which for a copy means
 * telling every reader that a `published` face is a thing this entity could
 * have. Absence is also what the rest of this app does (`v-if="canUpdate"` on
 * Edit, `v-if="canDelete"` on Delete), so a greyed-out Publish would be the
 * odd one out.
 *
 * `allowed` is a HINT, never the boundary — the invoke re-authorizes through
 * the kernel. Hiding a denied offer is a UI courtesy, not a security control,
 * and nothing here depends on it being right.
 *
 * ## One offer is a button; several are a menu
 *
 * Not cosmetic. "Publish this policy" is a single act and deserves a single
 * click; a set of translate definitions is a CHOICE, and a row of sibling
 * buttons reads as several unrelated actions rather than one decision. The
 * split is on the count of ALLOWED offers, so a principal permitted only one
 * of several translations gets the button — which is correct, because for
 * them it is not a choice.
 */
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import type { CopyOffer } from '@/types'

const props = defineProps<{
  offers?: CopyOffer[]
  /** Disables every control while an invoke is in flight. */
  busy?: boolean
}>()

const emit = defineEmits<{ invoke: [offer: CopyOffer] }>()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)

// Absent `_copies` (server computed no offers) and `[]` (this face genuinely
// offers none) both render nothing, so they need no distinction HERE — but
// they are different claims, and the type keeps them apart for callers that
// do care. See Entity._copies.
const allowed = computed(() => (props.offers ?? []).filter((o) => o.allowed))

// A single-offer label stands alone on a button, so it needs to name the act
// ("Publish this policy"). Inside a menu the group already supplies context.
const single = computed(() => (allowed.value.length === 1 ? allowed.value[0] : null))

function labelOf(o: CopyOffer): string {
  // `label` is operator-configured and optional; the definition NAME is the
  // documented fallback (`promote-control` reads as an action already).
  return o.label || o.name
}

onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))

function onDocClick(e: MouseEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) open.value = false
}

function choose(o: CopyOffer) {
  open.value = false
  emit('invoke', o)
}
</script>

<template>
  <button
    v-if="single"
    class="btn btn-secondary copy-single"
    :disabled="busy"
    @click="choose(single)"
  >
    {{ labelOf(single) }}
  </button>

  <div v-else-if="allowed.length > 1" ref="rootRef" class="copy-menu">
    <button
      class="btn btn-secondary"
      :disabled="busy"
      :aria-expanded="open"
      aria-haspopup="menu"
      @click="open = !open"
    >
      Copy to ▾
    </button>
    <ul v-if="open" class="copy-menu-list" role="menu">
      <li v-for="o in allowed" :key="o.name" role="none">
        <button
          role="menuitem"
          class="copy-menu-item"
          :title="o.targetFace"
          @click="choose(o)"
        >
          {{ labelOf(o) }}
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.copy-menu {
  position: relative;
  display: inline-block;
}

.copy-menu-list {
  position: absolute;
  right: 0;
  top: calc(100% + 4px);
  z-index: 20;
  min-width: 12rem;
  margin: 0;
  padding: 0.25rem;
  list-style: none;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.copy-menu-item {
  display: block;
  width: 100%;
  padding: 0.4rem 0.6rem;
  text-align: left;
  background: none;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-color);
  font-size: 0.9rem;
}

.copy-menu-item:hover,
.copy-menu-item:focus {
  background: var(--hover-bg);
}
</style>
