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
import { computed } from 'vue'
import type { Face } from '@/types'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'

const props = defineProps<{
  faces?: Face[]
}>()

const emit = defineEmits<{ select: [face: Face] }>()


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



function choose(f: Face) {
  emit('select', f)
}
</script>

<template>
  <RlButton v-if="single" variant="secondary" class="face-single" @click="choose(single)">
    View {{ labelOf(single) }}
  </RlButton>

  <!--
    RlMenu owns the open state, the click-outside listener, arrow-key
    navigation and the panel's position (teleported, so a scrolling ancestor
    cannot clip it). It closes itself on a click inside the panel.
  -->
  <RlMenu v-else-if="others.length > 1" class="face-menu">
    <template #trigger="{ toggle, attrs }">
      <RlButton variant="secondary" v-bind="attrs" @click="toggle">
        View
        <template #trailing>
          <RlIcon name="chevron-down" :size="14" aria-hidden="true" />
        </template>
      </RlButton>
    </template>

    <RlMenuItem v-for="f in others" :key="f.face" @click="choose(f)">
      {{ labelOf(f) }}
    </RlMenuItem>
  </RlMenu>
</template>

