<script setup lang="ts">
withDefaults(defineProps<{ width?: number }>(), { width: 1200 });
</script>

<template>
  <div class="page-container" :style="{ maxWidth: `${width}px` }">
    <slot />
  </div>
</template>

<style scoped>
.page-container {
  width: 100%;
  margin: 0 auto;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

@media (max-width: 640px) {
  .page-container {
    padding: 16px;
    gap: 14px;
  }
}
</style>
