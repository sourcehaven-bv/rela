<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import type { SelectOption } from 'rela-components/components/form/types'
import { setUserLanguage } from '@/i18n'
import { automaticLanguage, chosenLanguage } from '@/i18n/locale'
import { isUiLanguage, type UiLanguage } from '@/i18n/language'

/**
 * The interface-language choice on the Settings page (TKT-ZL1QAO).
 *
 * Its own component so the Settings page carries one line for it. The card
 * markup repeats the page's `settings-card` shape because scoped styles do not
 * reach into a child component.
 *
 * Language names are endonyms ("Nederlands" in every language), so a user who
 * landed in a language they cannot read can still find their own.
 */
const { t } = useI18n()

// Literal keys, so the unused-key check can see them.
const LANGUAGE_NAMES: Record<UiLanguage, `language.${UiLanguage}`> = {
  en: 'language.en',
  nl: 'language.nl',
}

const options = computed<SelectOption[]>(() => [
  {
    value: '',
    label: t('settings.language.automatic', {
      language: t(LANGUAGE_NAMES[automaticLanguage.value.language]),
    }),
  },
  ...(Object.keys(LANGUAGE_NAMES) as UiLanguage[]).map((lang) => ({
    value: lang,
    label: t(LANGUAGE_NAMES[lang]),
  })),
])

const choice = computed<string>({
  get: () => chosenLanguage.value ?? '',
  set: (value: string) => setUserLanguage(isUiLanguage(value) ? value : null),
})
</script>

<template>
  <div class="language-card" data-testid="language-setting">
    <h3>{{ t('settings.language.title') }}</h3>
    <p class="language-description">{{ t('settings.language.description') }}</p>
    <div class="language-row">
      <RlSelect
        v-model="choice"
        class="language-select"
        :label="t('settings.language.label')"
        :options="options"
      />
    </div>
  </div>
</template>

<style scoped>
.language-card {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}

.language-card h3 {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
}

.language-description {
  color: var(--rl-color-text-muted);
  font-size: 13px;
  margin: 0 0 16px;
}

.language-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: var(--rl-color-bg-hover);
  border-radius: 6px;
}
</style>
