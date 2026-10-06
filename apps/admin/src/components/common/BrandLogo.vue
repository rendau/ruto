<script setup lang="ts">
withDefaults(defineProps<{ size?: number; showText?: boolean }>(), {
  size: 22,
  showText: true
});
</script>

<template>
  <span class="brand" :style="{ fontSize: `${size}px` }">
    <span class="brand__word" role="img" aria-label="ruto">
      <span aria-hidden="true">rut</span>
      <svg class="brand__ring" viewBox="0 0 32 32" aria-hidden="true">
        <circle cx="16" cy="16" r="11" fill="none" stroke="currentColor" stroke-width="7" />
        <circle cx="16" cy="16" r="3.2" fill="currentColor" />
      </svg>
    </span>
    <span v-if="showText" class="brand__suffix">admin</span>
  </span>
</template>

<style scoped>
.brand {
  display: inline-flex;
  align-items: baseline;
  gap: 0.4em;
  line-height: 1;
  white-space: nowrap;
}

.brand__word {
  font-weight: 700;
  letter-spacing: -0.035em;
  color: var(--c-primary);
}

.brand__ring {
  width: 0.6em;
  height: 0.6em;
  margin-left: 0.02em;
  vertical-align: -0.045em;
}

.brand__suffix {
  font-size: 0.6em;
  font-weight: 600;
  letter-spacing: 0.01em;
  color: var(--c-text-3);
}
</style>
