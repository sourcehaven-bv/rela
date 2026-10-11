<script setup lang="ts">
/**
 * The review step of the Configure mockups: every unsaved change in the
 * draft, in the reader's terms, and the data migration saving generates.
 *
 * Additive changes (a new property, option, field or column) fit existing
 * records as they are and need no migration. A change that does not fit them,
 * such as removing an option records still use, makes saving also write a
 * migration that updates those records once. The review asks only for what
 * the draft cannot decide on its own: here, where the Blocked tickets go.
 *
 * A fixture for the mockup stories, not part of the library's surface. The
 * draft is fixed sample data.
 */
import { computed, ref } from 'vue'
import RlDrawer from '../components/overlay/RlDrawer.vue'
import RlButton from '../components/common/RlButton.vue'
import RlButtonGroup from '../components/common/RlButtonGroup.vue'
import RlChangeList from '../components/data/RlChangeList.vue'
import RlChangeItem from '../components/data/RlChangeItem.vue'
import RlHeading from '../components/common/RlHeading.vue'
import RlTag from '../components/common/RlTag.vue'
import RlText from '../components/common/RlText.vue'
import RlSelect from '../components/form/RlSelect.vue'
import RlTextField from '../components/form/RlTextField.vue'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; save: [summary: string]; discard: [] }>()

/** Where the 3 tickets that are Blocked go once Blocked is removed. */
const blockedTo = ref('')
const statusOptions = [
  { value: 'On hold', label: 'On hold (new)' },
  { value: 'Planning', label: 'Planning' },
  { value: 'In progress', label: 'In progress' },
  { value: 'Backlog', label: 'Backlog' },
]
const blockedTickets = [
  { id: 'TKT-4QK2ZD', title: 'Wait for upstream neoq fix' },
  { id: 'TKT-8M1XRA', title: 'Postgres listener reconnect storm' },
  { id: 'TKT-J0W7EC', title: 'Calendar feed for shared spaces' },
]
const showTickets = ref(false)

const migrationName = ref('Remove the Blocked status')

const ready = computed(() => blockedTo.value !== '')
const saving = ref(false)

function save() {
  saving.value = true
  setTimeout(() => {
    saving.value = false
    emit('save', `The migration moved 3 tickets to ${blockedTo.value}.`)
  }, 900)
}
</script>

<template>
  <RlDrawer title="Review changes" size="lg" :open="open" @close="emit('close')">
    <div class="review">
      <RlChangeList title="Ticket" :count="1">
        <RlChangeItem kind="added" label="Property Due" detail="Date, optional" as="button" />
      </RlChangeList>

      <RlChangeList title="Ticket status" :count="2">
        <RlChangeItem kind="added" label="Option On hold" as="button">
          <template #after><RlTag label="Amber" color="amber" /></template>
        </RlChangeItem>
        <RlChangeItem kind="removed" label="Option Blocked" detail="used by 3 tickets" as="button" />
      </RlChangeList>

      <RlChangeList title="Edit ticket form" :count="1">
        <RlChangeItem kind="added" label="Field Due" detail="after Priority, half width" as="button" />
      </RlChangeList>

      <RlChangeList title="All tickets list" :count="1">
        <RlChangeItem kind="added" label="Column Due" detail="sortable" as="button" />
      </RlChangeList>

      <section class="review__migration" aria-labelledby="review-migration">
        <div class="review__intro">
          <RlHeading id="review-migration" :level="3" size="md">Data migration</RlHeading>
          <RlText size="sm" tone="muted" as="p" class="review__para">
            Removing Blocked does not fit 3 tickets that use it. Saving also writes a migration that updates them
            once and keeps a record of it in the migration history. The other 4 changes fit existing records as
            they are.
          </RlText>
        </div>

        <RlSelect
          v-model="blockedTo"
          label="Tickets that are Blocked move to"
          :options="statusOptions"
          placeholder="Choose a status"
        />

        <RlChangeList title="Steps" :level="4" :count="2">
          <RlChangeItem
            kind="changed"
            label="Status of 3 tickets"
            before="Blocked"
            :after="blockedTo || 'not chosen yet'"
          />
          <RlChangeItem kind="removed" label="Option Blocked" detail="from Ticket status" />
          <template #actions>
            <RlButton size="sm" variant="ghost" @click="showTickets = !showTickets">
              {{ showTickets ? 'Hide the tickets' : 'Show the 3 tickets' }}
            </RlButton>
          </template>
        </RlChangeList>

        <ul v-if="showTickets" class="review__tickets">
          <li v-for="ticket in blockedTickets" :key="ticket.id">
            <RlText size="sm" tone="muted">{{ ticket.id }}</RlText>
            <RlText size="sm">{{ ticket.title }}</RlText>
          </li>
        </ul>

        <RlTextField
          v-model="migrationName"
          label="Name in the migration history"
          hint="Says why the records changed, for whoever reads the history later."
        />
      </section>
    </div>

    <template #actions>
      <RlText v-if="!ready" size="sm" tone="muted" style="margin-right: auto">
        Choose a status for the Blocked tickets first.
      </RlText>
      <RlButtonGroup>
        <RlButton variant="ghost" tone="danger" @click="emit('discard')">Discard all</RlButton>
        <RlButton
          variant="primary"
          :disabled="!ready"
          :loading="saving"
          pending-label="Saving and migrating…"
          @click="save"
        >
          Save and migrate 3 tickets
        </RlButton>
      </RlButtonGroup>
    </template>
  </RlDrawer>
</template>

<style scoped>
.review { display: flex; flex-direction: column; gap: var(--rl-space-6); }

.review__migration {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
  padding-top: var(--rl-space-6);
  border-top: 1px solid var(--rl-color-border);
}

.review__intro { display: flex; flex-direction: column; gap: var(--rl-space-2); }
.review__para { margin: 0; }

.review__tickets {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}
.review__tickets li { display: flex; gap: var(--rl-space-3); }
</style>
