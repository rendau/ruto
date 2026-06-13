<script setup lang="ts">
import { NCard } from "naive-ui";

defineProps<{ title?: string; description?: string }>();
</script>

<template>
  <NCard class="section-card" :bordered="true">
    <template v-if="title || $slots.extra || $slots.title" #header>
      <div class="section-card__header">
        <div class="section-card__heading">
          <slot name="title">
            <span class="section-card__title">{{ title }}</span>
          </slot>
          <p v-if="description" class="section-card__desc muted">{{ description }}</p>
        </div>
        <div v-if="$slots.extra" class="section-card__extra">
          <slot name="extra" />
        </div>
      </div>
    </template>
    <slot />
  </NCard>
</template>

<style scoped>
.section-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.section-card__heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.section-card__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--c-text);
}

.section-card__desc {
  margin: 0;
  font-size: 12.5px;
}

.section-card__extra {
  flex-shrink: 0;
}
</style>
