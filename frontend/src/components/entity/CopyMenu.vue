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
import { computed } from 'vue'
import type { CopyOffer } from '@/types'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'

const props = defineProps<{
  offers?: CopyOffer[]
  /** Disables every control while an invoke is in flight. */
  busy?: boolean
}>()

const emit = defineEmits<{ invoke: [offer: CopyOffer] }>()


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



function choose(o: CopyOffer) {
  emit('invoke', o)
}
</script>

<template>
  <RlButton
    v-if="single"
    variant="secondary"
    class="copy-single"
    :disabled="busy"
    @click="choose(single)"
  >
    {{ labelOf(single) }}
  </RlButton>

  <!-- See the note in FaceMenu: RlMenu owns the open state and positioning. -->
  <RlMenu v-else-if="allowed.length > 1" class="copy-menu">
    <template #trigger="{ toggle, attrs }">
      <RlButton variant="secondary" :disabled="busy" v-bind="attrs" @click="toggle">
        Copy to
        <template #trailing>
          <RlIcon name="chevron-down" :size="14" aria-hidden="true" />
        </template>
      </RlButton>
    </template>

    <RlMenuItem v-for="o in allowed" :key="o.name" :title="o.targetFace" @click="choose(o)">
      {{ labelOf(o) }}
    </RlMenuItem>
  </RlMenu>
</template>

