<script setup lang="ts">
withDefaults(defineProps<{ size?: number; showText?: boolean }>(), {
  size: 28,
  showText: true
});
</script>

<template>
  <span class="brand">
    <img class="brand__logo" src="/favicon.svg" alt="Ruto" :width="size" :height="size" />
    <span v-if="showText" class="brand__text">
      Ruto<span class="brand__accent">Admin</span>
    </span>
  </span>
</template>

<style scoped>
.brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.brand__logo {
  border-radius: 7px;
  display: block;
}

.brand__text {
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.01em;
  color: var(--c-text);
}

.brand__accent {
  margin-left: 4px;
  color: var(--c-text-3);
  font-weight: 600;
}
</style>
