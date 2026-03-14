<script setup lang="ts">
import { useSchemaStore } from '@/stores'

const schemaStore = useSchemaStore()
</script>

<template>
  <div class="settings-view">
    <h1>Settings</h1>

    <div class="settings-card">
      <h3>User Defaults</h3>
      <p class="description">
        Configure default values for entity creation. User defaults are stored locally
        and are not shared with other users.
      </p>
      <p class="placeholder">
        Settings configuration will be available in a future update.
        For now, defaults can be configured in <code>.rela/user-defaults.yaml</code>.
      </p>
    </div>

    <div class="settings-card">
      <h3>Application Info</h3>
      <div class="info-grid">
        <div class="info-row">
          <span class="info-label">App Name</span>
          <span class="info-value">{{ schemaStore.app.name }}</span>
        </div>
        <div class="info-row" v-if="schemaStore.app.description">
          <span class="info-label">Description</span>
          <span class="info-value">{{ schemaStore.app.description }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Entity Types</span>
          <span class="info-value">{{ schemaStore.entityTypes.size }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Relation Types</span>
          <span class="info-value">{{ schemaStore.relationTypes.size }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Forms</span>
          <span class="info-value">{{ schemaStore.forms.size }}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Lists</span>
          <span class="info-value">{{ schemaStore.lists.size }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-view {
  max-width: 700px;
}

h1 {
  margin-bottom: 24px;
}

.settings-card {
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}

.settings-card h3 {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
}

.description {
  color: #64748b;
  font-size: 14px;
  margin: 0 0 16px;
}

.placeholder {
  color: #64748b;
  font-size: 14px;
  font-style: italic;
  background: #f8fafc;
  padding: 16px;
  border-radius: 6px;
  margin: 0;
}

.placeholder code {
  background: #e2e8f0;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

.info-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 8px 12px;
  background: #f8fafc;
  border-radius: 6px;
}

.info-label {
  color: #64748b;
  font-size: 14px;
}

.info-value {
  font-size: 14px;
  font-weight: 500;
  color: #1e293b;
}
</style>
