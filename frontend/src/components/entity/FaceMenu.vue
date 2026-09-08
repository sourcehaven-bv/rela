<script setup lang="ts">
/**
 * Renders the OTHER content states this entity has, as a way to switch between
 * them: "View published" on a draft policy, a language menu on a blog post
 * that has translations.
 *
 * ## Existence, not permission
 *
 * `_faces` reports which faces the entity HAS. It carries no readability flag
 * and this component does not ask for one: world-read is a GLOBAL, role-level
 * grant the client already holds from `/_schema`.worlds, so re-answering it
 * per face would be a per-instance check for a per-principal question.
 *
 * A face the principal may not read still renders here, and clicking it lands
 * on the ordinary row gate — the same answer a typed URL gives. That is
 * deliberate: face names are operator-authored config and public, so the
 * button's presence discloses nothing the schema endpoint did not.
 *
 * ## One face is a button; several are a menu
 *
 * Same split as CopyMenu, for the same reason. "View published" is one
 * destination and deserves one click; a set of translations is a CHOICE, and a
 * row of sibling buttons reads as several unrelated actions rather than one
 * decision.
 *
 * ## Why it renders on every screen that has faces
 *
 * Not gated on world-boundness. A reader on the published face wants the way
 * back to the draft, and an author on the draft wants to see what readers see;
 * the multilingual case has no privileged direction at all. Suppressing it
 * under a world would make the English post a dead end.
 */
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import type { Face } from '@/types'

const props = defineProps<{
  faces?: Face[]
}>()

const emit = defineEmits<{ select: [face: Face] }>()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)

// The server computes `_faces` on the record it SERVED — under
// `?world=published` that is the published face — and excludes it, so the
// menu never offers the page you are on. A client-side `current` filter used
// to duplicate that and, with no value given, silently dropped the default
// face (its empty coordinate equalled the default of the prop).
const others = computed(() => props.faces ?? [])

const single = computed(() => (others.value.length === 1 ? others.value[0] : null))

function labelOf(f: Face): string {
  // The declared face name is operator-authored (`published`, `nl`), so it
  // is the honest display string. An empty face is the default face.
  return f.label || f.face || 'default'
}

onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))

function onDocClick(e: MouseEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) open.value = false
}

function choose(f: Face) {
  open.value = false
  emit('select', f)
}
</script>

<template>
  <button
    v-if="single"
    class="btn btn-secondary face-single"
    @click="choose(single)"
  >
    View {{ labelOf(single) }}
  </button>

  <div v-else-if="others.length > 1" ref="rootRef" class="face-menu">
    <button
      class="btn btn-secondary"
      :aria-expanded="open"
      aria-haspopup="menu"
      @click="open = !open"
    >
      View ▾
    </button>
    <ul v-if="open" class="face-menu-list" role="menu">
      <li v-for="f in others" :key="f.face" role="none">
        <button role="menuitem" class="face-menu-item" @click="choose(f)">
          {{ labelOf(f) }}
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.face-menu {
  position: relative;
  display: inline-block;
}
.face-menu-list {
  position: absolute;
  right: 0;
  z-index: 20;
  min-width: 12rem;
  margin: 0.25rem 0 0;
  padding: 0.25rem 0;
  list-style: none;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  box-shadow: 0 6px 18px rgb(0 0 0 / 18%);
}
.face-menu-item {
  display: block;
  width: 100%;
  padding: 0.4rem 0.85rem;
  border: 0;
  background: none;
  text-align: left;
  cursor: pointer;
  color: var(--text-color);
  font: inherit;
}
.face-menu-item:hover {
  background: var(--hover-bg);
}
</style>
