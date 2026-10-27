<script setup lang="ts">
import { computed } from "vue";
import { NTag } from "naive-ui";
import type { Compression } from "@/api/types";

const props = defineProps<{ compression: Compression }>();

const backendLabel = computed(() => {
  switch (props.compression.backend) {
    case "identity":
      return "plain";
    case "client":
      return "as the client asks";
    default:
      return "inherit";
  }
});

const responseLabel = computed(() => props.compression.response || "inherit");
</script>

<template>
  <div class="compression-summary">
    <div class="compression-summary__row">
      <span class="compression-summary__label section-label">Backend</span>
      <NTag size="tiny" :bordered="false" round>{{ backendLabel }}</NTag>
      <span class="compression-summary__label section-label">Response</span>
      <NTag size="tiny" :bordered="false" round>{{ responseLabel }}</NTag>
      <template v-if="compression.min_size">
        <span class="compression-summary__label section-label">Min size</span>
        <NTag size="tiny" :bordered="false" round>{{ compression.min_size }} B</NTag>
      </template>
    </div>
    <div class="compression-summary__row">
      <span class="compression-summary__label section-label">Algorithms</span>
      <template v-if="compression.algorithms.length">
        <NTag
          v-for="name in compression.algorithms"
          :key="name"
          size="tiny"
          type="primary"
          :bordered="false"
        >
          {{ name }}
        </NTag>
      </template>
      <span v-else class="muted">inherit</span>
    </div>
    <div v-if="compression.content_types.length" class="compression-summary__row">
      <span class="compression-summary__label section-label">Types</span>
      <NTag v-for="ct in compression.content_types" :key="ct" size="tiny" :bordered="false">
        {{ ct }}
      </NTag>
    </div>
  </div>
</template>

<style scoped>
.compression-summary {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.compression-summary__row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.compression-summary__label {
  margin-right: 2px;
}
</style>
