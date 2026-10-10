<script setup lang="ts">
/**
 * Mockup: the description section of a detail page, with suggesting.
 *
 * Story-only. A pen beside the heading opens a menu, Edit or Suggest
 * changes, and the choice opens the editor, so the mode is picked per edit.
 * A sticky Edit | Suggest switch was considered and rejected (TKT-H3ILCP).
 * Clicking the text itself still edits directly, as inline editing does
 * elsewhere. A reader who may not edit gets no choice: the pen and a click
 * both open the editor in suggesting mode.
 *
 * Suggested changes are drawn inline. Clicking one opens its card.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import RlButton from '../components/common/RlButton.vue'
import RlHeading from '../components/common/RlHeading.vue'
import RlIconButton from '../components/common/RlIconButton.vue'
import RlMarkdownEditor from '../components/editor/RlMarkdownEditor.vue'
import RlMenu from '../components/overlay/RlMenu.vue'
import RlMenuItem from '../components/overlay/RlMenuItem.vue'
import RlTrackedBody from './RlTrackedBody.vue'
import { diffMarkdown, splitPrefix, stripMarkers } from './trackChanges'
import { inline, initials, useTrackedChanges, type ChangeSet } from './useTrackedChanges'

type Mode = 'edit' | 'suggest'

const props = withDefaults(
  defineProps<{
    body: string
    changeSets?: ChangeSet[]
    /** False for a reader who may comment but not edit. */
    canEdit?: boolean
  }>(),
  { changeSets: () => [], canEdit: true }
)

const body = ref(props.body)
const posted = ref<ChangeSet[]>([...props.changeSets])
/* With nothing posted, diff the body against itself so it still renders. */
const sets = computed<ChangeSet[]>(() =>
  posted.value.length
    ? posted.value
    : [{ id: 'none', author: '', when: '', base: body.value, edited: body.value }]
)
const { status, current, diff, blocks, pending, label, setStatus } = useTrackedChanges(sets)

const editing = ref<Mode | null>(null)
const draft = ref('')
const draftChanges = computed(() => diffMarkdown(body.value, draft.value).changes.length)

function open(mode: Mode) {
  picked.value = null
  draft.value = body.value
  editing.value = props.canEdit ? mode : 'suggest'
}

function clickBody(event: MouseEvent) {
  if ((event.target as HTMLElement).closest('.tc__mark')) return
  open('edit')
}

function finish() {
  if (editing.value === 'edit') {
    body.value = draft.value
    posted.value = []
  } else if (draftChanges.value) {
    posted.value = [
      {
        id: `you${Date.now()}`,
        author: 'You',
        when: 'just now',
        base: body.value,
        edited: draft.value,
      },
    ]
  }
  editing.value = null
}

/* The card for one change, anchored under the clicked mark. */
const root = ref<HTMLElement | null>(null)
const picked = ref<{ id: string; top: number; left: number } | null>(null)
const pickedChange = computed(() => diff.value.changes.find((c) => c.id === picked.value?.id))

function pick(id: string, event: MouseEvent) {
  const mark = (event.target as HTMLElement).closest('.tc__mark') ?? (event.target as HTMLElement)
  const r = mark.getBoundingClientRect()
  const base = root.value!.getBoundingClientRect()
  picked.value = {
    id,
    top: r.bottom - base.top + 6,
    left: Math.min(r.left - base.left, base.width - 340),
  }
}

function openFirst() {
  const first = pending.value[0]
  if (!first) return
  void nextTick(() => {
    const el = root.value?.querySelector<HTMLElement>(`.tc__mark`)
    el?.click()
  })
}

function decide(s: 'accepted' | 'rejected') {
  if (picked.value) setStatus(picked.value.id, s)
  const next = pending.value[0]
  picked.value = null
  if (next) openFirst()
}

function onDocClick(event: MouseEvent) {
  const t = event.target as HTMLElement
  if (!t.closest('.sd__card') && !t.closest('.tc__mark')) picked.value = null
}
function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') picked.value = null
}
onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKey)
})

const author = computed(() => current.value?.author ?? '')
</script>

<template>
  <section ref="root" class="sd">
    <header class="sd__head">
      <RlHeading :level="2" size="md" weight="normal" tone="muted" line-height="normal"
        >Description</RlHeading
      >
      <button
        v-if="pending.length && !editing"
        type="button"
        class="sd__count"
        @click.stop="openFirst"
      >
        {{ pending.length === 1 ? '1 suggestion' : `${pending.length} suggestions` }}
      </button>
      <span class="sd__spacer" />

      <template v-if="!editing">
        <!-- A reader who may not edit: one action, no choice. -->
        <RlButton v-if="!canEdit" size="sm" variant="ghost" icon="edit" @click="open('suggest')"
          >Suggest changes</RlButton
        >

        <RlMenu v-else align="end">
          <template #trigger="{ toggle, attrs }">
            <RlIconButton
              icon="edit"
              label="Edit description"
              :size="16"
              v-bind="attrs"
              @click="toggle"
            />
          </template>
          <RlMenuItem icon="edit" @click="open('edit')">
            Edit
            <span class="sd__hint">Changes are saved directly</span>
          </RlMenuItem>
          <RlMenuItem icon="message-square" @click="open('suggest')">
            Suggest changes
            <span class="sd__hint">Others review them first</span>
          </RlMenuItem>
        </RlMenu>
      </template>
    </header>

    <div v-if="editing" class="sd__editor" :class="`sd__editor--${editing}`">
      <div class="sd__editbar">
        <span class="sd__badge" :class="`sd__badge--${editing}`">
          <span class="sd__dot" aria-hidden="true" />
          <template v-if="editing === 'edit'">Editing</template>
          <template v-else-if="canEdit"
            >Suggesting. Changes are posted for review, not saved.</template
          >
          <template v-else>You can't edit this. Your changes are posted as suggestions.</template>
        </span>
        <span class="sd__spacer" />
        <RlButton size="sm" variant="ghost" @click="editing = null">Cancel</RlButton>
        <RlButton
          size="sm"
          variant="primary"
          :disabled="editing === 'suggest' && draftChanges === 0"
          @click="finish"
        >
          <template v-if="editing === 'edit'">Save</template>
          <template v-else>{{
            draftChanges === 1 ? 'Post 1 suggestion' : `Post ${draftChanges} suggestions`
          }}</template>
        </RlButton>
      </div>
      <RlMarkdownEditor v-model="draft" />
    </div>

    <div v-else class="sd__body" @click="clickBody">
      <RlTrackedBody dense :blocks="blocks" :active="picked?.id" @pick="pick" />
    </div>

    <div
      v-if="picked && pickedChange"
      class="sd__card"
      role="dialog"
      :aria-label="label(pickedChange)"
      :style="{ top: `${picked.top}px`, left: `${Math.max(0, picked.left)}px` }"
    >
      <div class="sd__card-head">
        <span class="sd__avatar">{{ initials(author) }}</span>
        <div>
          <strong>{{ author }}</strong>
          <div class="sd__muted">{{ label(pickedChange) }} · {{ current?.when }}</div>
        </div>
      </div>
      <div class="sd__card-diff">
        <del v-if="pickedChange.del && pickedChange.kind !== 'formatted'">{{
          stripMarkers(splitPrefix(pickedChange.del)[1])
        }}</del>
        <ins v-if="pickedChange.ins">
          <template v-for="(piece, p) in inline(splitPrefix(pickedChange.ins)[1])" :key="p">
            <strong v-if="piece.fmt === 'b'">{{ piece.text }}</strong>
            <code v-else-if="piece.fmt === 'code'">{{ piece.text }}</code>
            <template v-else>{{ piece.text }}</template>
          </template>
        </ins>
      </div>
      <div class="sd__card-actions">
        <template v-if="canEdit && status[pickedChange.id] === 'pending'">
          <RlButton size="sm" variant="secondary" icon="check" @click="decide('accepted')"
            >Accept</RlButton
          >
          <RlButton size="sm" variant="ghost" icon="x" @click="decide('rejected')">Reject</RlButton>
        </template>
        <RlButton size="sm" variant="ghost" icon="message-square">Reply</RlButton>
        <span class="sd__spacer" />
        <span class="sd__muted">{{ pending.length }} open</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.sd {
  position: relative;
  margin-top: var(--rl-space-8);
}
.sd__head {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-height: 32px;
  margin-bottom: var(--rl-space-3);
}
.sd__spacer {
  flex: 1;
}
.sd__count {
  border: 0;
  padding: 2px 8px;
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-success-bg);
  color: var(--rl-color-success-fg);
  font: inherit;
  font-size: var(--rl-font-size-xs);
  cursor: pointer;
}
.sd__hint {
  display: block;
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-muted);
}
.sd__body {
  cursor: text;
  border-radius: var(--rl-radius-md);
  margin: 0 calc(-1 * var(--rl-space-2));
  padding: var(--rl-space-1) var(--rl-space-2);
}
.sd__body:hover {
  background: var(--rl-color-bg-hover);
}
.sd__editor {
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  overflow: hidden;
}
.sd__editor--suggest {
  border-color: var(--rl-color-status-green);
  box-shadow: inset 3px 0 0 var(--rl-color-status-green);
}
.sd__editbar {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-1) var(--rl-space-2) var(--rl-space-1) var(--rl-space-3);
  background: var(--rl-color-bg-sunken);
  border-bottom: 1px solid var(--rl-color-border);
}
.sd__badge {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-muted);
}
.sd__badge--suggest {
  color: var(--rl-color-success-fg);
}
.sd__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--rl-color-status-grey);
}
.sd__badge--suggest .sd__dot {
  background: var(--rl-color-status-green);
}
.sd__card {
  position: absolute;
  z-index: var(--rl-z-menu);
  width: 330px;
  padding: var(--rl-space-3);
  border: 1px solid var(--rl-color-border-raised);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg-raised);
  box-shadow: var(--rl-shadow-lg);
  font-size: var(--rl-font-size-sm);
}
.sd__card-head {
  display: flex;
  gap: var(--rl-space-2);
  align-items: center;
}
.sd__avatar {
  display: inline-grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--rl-color-status-blue);
  color: var(--rl-color-text-inverse);
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-semibold);
}
.sd__muted {
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
}
.sd__card-diff {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-1);
  margin: var(--rl-space-2) 0;
}
.sd__card-diff del {
  color: var(--rl-color-danger);
  background: var(--rl-color-danger-bg);
  padding: 0 2px;
}
.sd__card-diff ins {
  text-decoration: none;
  color: var(--rl-color-success-fg);
  background: var(--rl-color-success-bg);
  padding: 0 2px;
}
.sd__card-actions {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
}
</style>
