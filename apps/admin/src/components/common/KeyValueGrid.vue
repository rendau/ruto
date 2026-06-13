<script setup lang="ts">
import type { Variable } from "@/api/types";

withDefaults(
  defineProps<{ items: Variable[]; emptyText?: string; maskValues?: boolean }>(),
  { emptyText: "None", maskValues: false }
);
</script>

<template>
  <div class="kv-grid">
    <template v-if="items.length">
      <div v-for="(item, index) in items" :key="index" class="kv-grid__row">
        <code class="kv-grid__key">{{ item.key }}</code>
        <code class="kv-grid__value">{{ maskValues ? "••••••" : item.value || "—" }}</code>
      </div>
    </template>
    <span v-else class="muted kv-grid__empty">{{ emptyText }}</span>
  </div>
</template>

<style scoped>
.kv-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.kv-grid__row {
  display: grid;
  grid-template-columns: minmax(120px, 0.4fr) 1fr;
  gap: 12px;
  align-items: baseline;
}

.kv-grid__key {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--c-teal);
  overflow-wrap: anywhere;
}

.kv-grid__value {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.kv-grid__empty {
  font-size: 13px;
}

@media (max-width: 480px) {
  .kv-grid__row {
    grid-template-columns: 1fr;
    gap: 2px;
  }
}
</style>
