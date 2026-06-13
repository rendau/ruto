<script setup lang="ts">
defineProps<{ label: string }>();
</script>

<template>
  <div class="def-row">
    <div class="def-row__label">{{ label }}</div>
    <div class="def-row__value"><slot /></div>
  </div>
</template>

<style scoped>
.def-row {
  display: grid;
  grid-template-columns: 108px minmax(0, 1fr);
  align-items: start;
}

.def-row:not(:last-child) {
  border-bottom: 1px solid var(--c-border);
}

.def-row__label {
  padding: 9px 12px;
  background: var(--c-surface-2);
  border-right: 1px solid var(--c-border);
  color: var(--c-text-3);
  font-size: 12.5px;
  /* wrap only at spaces, never mid-word (avoids per-character wrapping) */
  word-break: normal;
  overflow-wrap: normal;
}

.def-row__value {
  padding: 9px 12px;
  min-width: 0;
  font-size: 13px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}
</style>
