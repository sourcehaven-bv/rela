<script setup lang="ts">
/**
 * Mockup: suggestions as tracked changes.
 *
 * Story-only. It shows the proposed review surface: the body with every
 * suggested change drawn inline, as Word and LibreOffice do, and a margin with
 * one card per change. Accepting or rejecting a change updates the body in
 * place. "Suggest edits" opens the ordinary editor; on posting, the edit is
 * diffed against the body and each hunk becomes a suggestion.
 */
import { computed, ref } from 'vue'
import RlButton from '../components/common/RlButton.vue'
import RlMarkdownEditor from '../components/editor/RlMarkdownEditor.vue'
import RlTrackedBody from './RlTrackedBody.vue'
import { diffMarkdown, splitPrefix, stripMarkers } from './trackChanges'
import { inline, initials, useTrackedChanges, type ChangeSet, type View } from './useTrackedChanges'

const props = withDefaults(
  defineProps<{
    /** The body as stored. */
    body: string
    /** Edit sessions already posted as suggestions, oldest first. */
    changeSets?: ChangeSet[]
    /** Open in suggesting mode instead of review. */
    startSuggesting?: boolean
  }>(),
  { changeSets: () => [], startSuggesting: false }
)

const sets = ref<ChangeSet[]>([...props.changeSets])
const view = ref<View>('markup')
const active = ref<string | null>(null)
const suggesting = ref(props.startSuggesting)
const draft = ref(props.body)
const { status, current, diff, blocks, pending, label, setStatus, resolveAll } = useTrackedChanges(
  sets,
  view
)
const draftChanges = computed(() => diffMarkdown(props.body, draft.value).changes.length)

function focusChange(id: string) {
  active.value = id
  document.getElementById(`tc-card-${id}`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
}

function startSuggesting() {
  draft.value = props.body
  suggesting.value = true
}

function post() {
  sets.value = [
    ...sets.value,
    {
      id: `you${sets.value.length + 1}`,
      author: 'You',
      when: 'just now',
      base: props.body,
      edited: draft.value,
    },
  ]
  suggesting.value = false
  view.value = 'markup'
}
</script>

<template>
  <div class="tc">
    <header class="tc__bar">
      <template v-if="!suggesting">
        <div class="tc__views" role="group" aria-label="Show">
          <RlButton
            v-for="v in ['markup', 'final', 'original'] as const"
            :key="v"
            size="sm"
            :variant="view === v ? 'secondary' : 'ghost'"
            :aria-pressed="view === v"
            @click="view = v"
          >
            {{ { markup: 'All changes', final: 'With changes', original: 'Original' }[v] }}
          </RlButton>
        </div>
        <span class="tc__spacer" />
        <RlButton size="sm" variant="primary" icon="edit" @click="startSuggesting"
          >Suggest edits</RlButton
        >
      </template>
      <template v-else>
        <span class="tc__mode">
          <span class="tc__dot" aria-hidden="true" />
          Suggesting. Your edits are posted as suggestions, not saved.
        </span>
        <span class="tc__spacer" />
        <RlButton size="sm" variant="ghost" @click="suggesting = false">Cancel</RlButton>
        <RlButton size="sm" variant="primary" :disabled="draftChanges === 0" @click="post">
          {{ draftChanges === 1 ? 'Post 1 suggestion' : `Post ${draftChanges} suggestions` }}
        </RlButton>
      </template>
    </header>

    <div v-if="suggesting" class="tc__editor">
      <RlMarkdownEditor v-model="draft" />
    </div>

    <div v-else class="tc__layout">
      <RlTrackedBody
        class="tc__body"
        :blocks="blocks"
        :active="active"
        @hover="active = $event"
        @pick="focusChange"
      />

      <aside class="tc__margin" aria-label="Suggested changes">
        <template v-if="current">
          <div class="tc__set">
            <span class="tc__avatar">{{ initials(current.author) }}</span>
            <div class="tc__set-text">
              <strong>{{ current.author }}</strong> suggested {{ diff.changes.length }} changes
              <div class="tc__muted">{{ current.when }} · {{ pending.length }} open</div>
            </div>
          </div>
          <div class="tc__set-actions">
            <RlButton
              size="sm"
              variant="secondary"
              icon="check"
              :disabled="!pending.length"
              @click="resolveAll('accepted')"
            >
              Accept all
            </RlButton>
            <RlButton
              size="sm"
              variant="ghost"
              icon="x"
              :disabled="!pending.length"
              @click="resolveAll('rejected')"
            >
              Reject all
            </RlButton>
          </div>
          <ol class="tc__cards">
            <li
              v-for="c in diff.changes"
              :id="`tc-card-${c.id}`"
              :key="c.id"
              class="tc__card"
              :class="[`tc__card--${status[c.id]}`, active === c.id && 'tc__card--active']"
              @mouseenter="active = c.id"
              @mouseleave="active = null"
            >
              <div class="tc__card-head">
                <span>{{ label(c) }}</span>
                <span v-if="status[c.id] !== 'pending'" class="tc__muted">
                  {{ status[c.id] === 'accepted' ? 'Accepted' : 'Rejected' }}
                </span>
              </div>
              <div class="tc__card-diff">
                <del v-if="c.del && c.kind !== 'formatted'">{{
                  stripMarkers(splitPrefix(c.del)[1])
                }}</del>
                <ins v-if="c.ins">
                  <template v-for="(piece, p) in inline(splitPrefix(c.ins)[1])" :key="p">
                    <strong v-if="piece.fmt === 'b'">{{ piece.text }}</strong>
                    <em v-else-if="piece.fmt === 'i'">{{ piece.text }}</em>
                    <code v-else-if="piece.fmt === 'code'">{{ piece.text }}</code>
                    <template v-else>{{ piece.text }}</template>
                  </template>
                </ins>
              </div>
              <div v-if="status[c.id] === 'pending'" class="tc__card-actions">
                <RlButton
                  size="sm"
                  variant="ghost"
                  icon="check"
                  @click="setStatus(c.id, 'accepted')"
                  >Accept</RlButton
                >
                <RlButton size="sm" variant="ghost" icon="x" @click="setStatus(c.id, 'rejected')"
                  >Reject</RlButton
                >
                <RlButton size="sm" variant="ghost" icon="message-square">Reply</RlButton>
              </div>
              <div v-else class="tc__card-actions">
                <RlButton size="sm" variant="ghost" @click="setStatus(c.id, 'pending')"
                  >Undo</RlButton
                >
              </div>
            </li>
          </ol>
        </template>
        <p v-else class="tc__muted">
          No suggestions. Use <strong>Suggest edits</strong> to propose changes.
        </p>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.tc {
  font-family: var(--rl-font-family);
  color: var(--rl-color-text);
  max-width: 1100px;
}
.tc__bar {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-2) var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg) var(--rl-radius-lg) 0 0;
  background: var(--rl-color-bg-sunken);
}
.tc__views {
  display: flex;
  gap: var(--rl-space-1);
}
.tc__spacer {
  flex: 1;
}
.tc__mode {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-success-fg);
}
.tc__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--rl-color-status-green);
}
.tc__editor {
  border: 1px solid var(--rl-color-border);
  border-top: 0;
  border-radius: 0 0 var(--rl-radius-lg) var(--rl-radius-lg);
  box-shadow: inset 3px 0 0 var(--rl-color-status-green);
}
.tc__layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  border: 1px solid var(--rl-color-border);
  border-top: 0;
  border-radius: 0 0 var(--rl-radius-lg) var(--rl-radius-lg);
}
.tc__margin {
  padding: var(--rl-space-4);
  border-left: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg-sunken);
  font-size: var(--rl-font-size-sm);
}
.tc__set {
  display: flex;
  gap: var(--rl-space-2);
  align-items: center;
}
.tc__avatar {
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
.tc__set-actions {
  display: flex;
  gap: var(--rl-space-1);
  margin: var(--rl-space-3) 0;
}
.tc__muted {
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
}
.tc__cards {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: var(--rl-space-2);
}
.tc__card {
  padding: var(--rl-space-2) var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-left: 3px solid var(--rl-color-status-blue);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-raised);
}
.tc__card--active {
  border-color: var(--rl-color-focus);
  box-shadow: var(--rl-shadow-sm);
}
.tc__card--accepted,
.tc__card--rejected {
  opacity: 0.6;
  border-left-color: var(--rl-color-status-grey);
}
.tc__card-head {
  display: flex;
  justify-content: space-between;
  font-weight: var(--rl-font-weight-medium);
}
.tc__card-diff {
  margin: var(--rl-space-1) 0;
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-1);
}
.tc__card-diff del {
  color: var(--rl-color-danger);
  background: var(--rl-color-danger-bg);
  padding: 0 2px;
}
.tc__card-diff ins {
  text-decoration: none;
  color: var(--rl-color-success-fg);
  background: var(--rl-color-success-bg);
  padding: 0 2px;
}
.tc__card-actions {
  display: flex;
  gap: var(--rl-space-1);
  margin-left: calc(-1 * var(--rl-space-2));
}
</style>
