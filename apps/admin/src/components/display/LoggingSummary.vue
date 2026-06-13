<script setup lang="ts">
import { computed } from "vue";
import { NTag } from "naive-ui";
import type { Logging } from "@/api/types";

const props = defineProps<{ logging: Logging }>();

const levelLabel = computed(() => {
  switch (props.logging.level) {
    case "all":
      return "all";
    case "error":
      return "error";
    case "none":
      return "don't log";
    default:
      return "inherit";
  }
});

const flags = computed(() => {
  const list: string[] = [];
  if (props.logging.headers) list.push("headers");
  if (props.logging.query_params) list.push("query params");
  if (props.logging.req_body) list.push("request body");
  if (props.logging.resp_body) list.push("response body");
  return list;
});
</script>

<template>
  <div class="logging-summary">
    <div class="logging-summary__row">
      <span class="logging-summary__label section-label">Level</span>
      <NTag size="tiny" :bordered="false" round>{{ levelLabel }}</NTag>
      <span class="logging-summary__label section-label">Mode</span>
      <NTag size="tiny" :bordered="false" round>{{ logging.mode }}</NTag>
    </div>
    <div class="logging-summary__row">
      <span class="logging-summary__label section-label">Captures</span>
      <template v-if="flags.length">
        <NTag v-for="flag in flags" :key="flag" size="tiny" type="primary" :bordered="false">
          {{ flag }}
        </NTag>
      </template>
      <span v-else class="muted">method &amp; path only</span>
    </div>
  </div>
</template>

<style scoped>
.logging-summary {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.logging-summary__row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.logging-summary__label {
  margin-right: 2px;
}
</style>
