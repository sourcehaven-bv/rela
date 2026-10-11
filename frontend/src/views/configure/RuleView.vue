<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlRadioGroup from 'rela-components/components/form/RlRadioGroup.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSplit from '@/components/configure/ConfigSplit.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import LockedNote from '@/components/configure/LockedNote.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypeLabel, entityTypeNames, rules, ruleTitle } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import type { TreeValue } from '@/configure/tree'

/**
 * A rule. Conditions are expressions typed as text; there is no builder yet.
 * Saving checks them, and a condition that does not compile comes back as a
 * problem on this screen.
 */
const props = defineProps<{ index: number }>()
const draft = useConfigDraftStore()
const router = useRouter()

const rule = computed(() => rules(draft.currentSchema).find((r) => r.index === props.index))
const path = computed(() => ['validations', props.index])
const typeOptions = computed(() =>
  entityTypeNames(draft.currentSchema).map((t) => ({
    value: t,
    label: entityTypeLabel(draft.currentSchema, t),
  }))
)
const locked = computed(() => rule.value?.usesScript === true)

function setting(key: string, value: TreeValue | undefined) {
  draft.setAt('schema', path.value, key, value)
}

function lines(value: string): string[] | undefined {
  const list = value
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
  return list.length ? list : undefined
}

function removeRule() {
  draft.removeAt('schema', ['validations'], props.index)
  void router.push(configureRoute.rules())
}

const severityOptions = [
  {
    value: 'error',
    label: 'Block saving',
    description: 'The record cannot be saved until it is fixed.',
  },
  {
    value: 'warning',
    label: 'Show a warning',
    description: 'The record saves; the warning shows on its page.',
  },
]
</script>

<template>
  <ConfigurePage
    :title="rule ? ruleTitle(rule) : 'Rule'"
    back-label="Rules"
    :back-to="configureRoute.rules()"
  >
    <template v-if="rule" #actions>
      <RlButton variant="ghost" tone="danger" icon="delete" @click="removeRule">Remove</RlButton>
    </template>
    <RlEmptyState v-if="!rule" title="No such rule" />
    <ConfigSplit v-else>
      <ConfigSection title="Rule">
        <LockedNote v-if="locked">
          This rule runs a script. Its name and description can be changed here; the script is
          changed in the project's files.
        </LockedNote>
        <RlTextField
          :model-value="rule.name"
          label="Name"
          required
          @update:model-value="setting('name', $event)"
        />
        <RlTextField
          :model-value="rule.description"
          label="Description"
          hint="Shown with the problem on a record that breaks the rule."
          @update:model-value="setting('description', $event)"
        />
        <RlSelect
          :model-value="rule.entityType"
          label="Applies to"
          :options="typeOptions"
          :disabled="locked"
          @update:model-value="setting('entity_type', $event)"
        />
        <template v-if="!locked">
          <RlTextField
            :model-value="rule.whenCondition"
            label="When"
            class="mono"
            hint="An expression. The rule is checked only for records that match it, such as entity.status == 'ready'."
            @update:model-value="setting('when_condition', $event)"
          />
          <RlTextField
            :model-value="rule.thenCondition"
            label="Then"
            class="mono"
            hint="What must be true of those records, such as entity.effort != nil."
            @update:model-value="setting('then_condition', $event)"
          />
          <RlTextarea
            v-if="rule.when.length || rule.then.length"
            :model-value="rule.when.join('\n')"
            label="When (filters, one per line)"
            class="mono"
            :rows="2"
            @update:model-value="setting('when', lines($event))"
          />
          <RlTextarea
            v-if="rule.when.length || rule.then.length"
            :model-value="rule.then.join('\n')"
            label="Then (filters, one per line)"
            class="mono"
            :rows="2"
            @update:model-value="setting('then', lines($event))"
          />
          <RlText v-if="rule.checksRelations" size="sm" tone="muted" as="p" class="para">
            This rule also checks how many linked records a record has. Those counts are shown on
            the relation's page.
          </RlText>
        </template>
        <RlRadioGroup
          :model-value="rule.severity"
          label="When a record breaks it"
          :options="severityOptions"
          @update:model-value="setting('severity', $event)"
        />
      </ConfigSection>
    </ConfigSplit>
  </ConfigurePage>
</template>

<style scoped>
.mono :deep(input),
.mono :deep(textarea) {
  font-family: var(--rl-font-family-mono);
}

.para {
  margin: 0;
}
</style>
