<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RlSwitch from 'rela-components/components/form/RlSwitch.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import { useMe } from '@/queries/me'
import { UNKNOWN_USER } from '@/utils/userToken'
import {
  DEFAULT_PREFS,
  notificationScope,
  requestPermission,
  setPrefs,
  storageWorks,
  support,
  usePrefsRef,
  watchSupport,
  type NotificationPrefs,
} from '@/utils/notificationPrefs'

/**
 * The browser-notification choice on the Settings page (TKT-48S88RB). Off by
 * default and kept per browser, project and user. Turning it on asks the
 * browser for permission inside the click, as browsers require.
 */
const { t } = useI18n()
const me = useMe()

watchSupport()

const scope = computed(() => {
  const user = me.data.value?.user
  return user && user !== UNKNOWN_USER ? notificationScope(user) : null
})

const prefs = computed<NotificationPrefs>(() =>
  scope.value ? usePrefsRef(scope.value).value : DEFAULT_PREFS
)

const blocked = computed(() => support.value === 'unsupported' || support.value === 'denied')

// Without storage every read would be a silent baseline: nothing would
// ever notify, so the switch refuses rather than show a dead "on".
const noStorage = ref(false)

const notice = computed(() => {
  if (!scope.value) return t('settings.notifications.signedOut')
  if (noStorage.value) return t('settings.notifications.noStorage')
  if (support.value === 'unsupported') return t('settings.notifications.unsupported')
  if (support.value === 'denied') return t('settings.notifications.denied')
  return ''
})

function update(patch: Partial<NotificationPrefs>) {
  if (scope.value) setPrefs(scope.value, { ...prefs.value, ...patch })
}

const enabled = computed<boolean>({
  get: () => prefs.value.enabled && support.value === 'granted',
  set: (on: boolean) => {
    if (!on) {
      update({ enabled: false })
      return
    }
    noStorage.value = !storageWorks()
    if (noStorage.value) return
    if (support.value === 'granted') {
      update({ enabled: true })
      return
    }
    void requestPermission().then((answer) => {
      if (answer === 'granted') update({ enabled: true })
    })
  },
})

const mentions = computed<boolean>({
  get: () => prefs.value.mentions,
  set: (on: boolean) => update({ mentions: on }),
})

const waiting = computed<boolean>({
  get: () => prefs.value.waiting,
  set: (on: boolean) => update({ waiting: on }),
})
</script>

<template>
  <div class="notification-card" data-testid="notification-setting">
    <h3>{{ t('settings.notifications.title') }}</h3>
    <p class="notification-description">{{ t('settings.notifications.description') }}</p>
    <div class="notification-row">
      <RlSwitch
        v-model="enabled"
        :label="t('settings.notifications.enable')"
        :disabled="!scope || blocked"
        data-testid="notification-enable"
      />
    </div>
    <p v-if="notice" class="notification-notice" data-testid="notification-notice">{{ notice }}</p>
    <div v-if="enabled" class="notification-kinds">
      <RlCheckbox v-model="mentions" :label="t('settings.notifications.mentions')" />
      <RlCheckbox v-model="waiting" :label="t('settings.notifications.waiting')" />
    </div>
  </div>
</template>

<style scoped>
.notification-card {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}

.notification-card h3 {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
}

.notification-description,
.notification-notice {
  color: var(--rl-color-text-muted);
  font-size: 13px;
  margin: 0 0 16px;
}

.notification-notice {
  margin: 12px 0 0;
}

.notification-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: var(--rl-color-bg-hover);
  border-radius: 6px;
}

.notification-kinds {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 12px;
  padding: 0 12px;
}
</style>
