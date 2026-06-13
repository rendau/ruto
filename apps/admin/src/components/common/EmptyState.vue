<script setup lang="ts">
import { NEmpty } from "naive-ui";

defineProps<{ description: string; size?: "small" | "medium" | "large" | "huge" }>();
</script>

<template>
  <div class="empty-state">
    <NEmpty :description="description" :size="size || 'large'">
      <template v-if="$slots.default" #extra>
        <slot />
      </template>
    </NEmpty>
  </div>
</template>

<style scoped>
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
}
</style>
