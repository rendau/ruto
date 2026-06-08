<script setup lang="ts">
import { computed } from "vue";
import type { Logging } from "../types/api";

const props = withDefaults(defineProps<{
  logging?: Logging | null;
  title?: string;
  hideMode?: boolean;
}>(), {
  title: "Logging"
});

const summary = computed(() => {
  const level = props.logging?.level || "default (error)";
  if (props.hideMode) {
    return level === "none" ? "level: none (logging disabled)" : `level: ${level}`;
  }
  const mode = (props.logging?.mode || "extend").toLowerCase() === "replace" ? "replace" : "extend";
  if (level === "none") {
    return `level: none (logging disabled), mode: ${mode}`;
  }
  return `level: ${level}, mode: ${mode}`;
});

const flags = computed(() => {
  const result: string[] = [];
  if (props.logging?.headers) result.push("headers");
  if (props.logging?.query_params) result.push("query params");
  if (props.logging?.req_body) result.push("request body");
  if (props.logging?.resp_body) result.push("response body");
  return result;
});
</script>

<template>
  <div class="logging-card">
    <h3 v-if="title">{{ title }}</h3>
    <p class="muted logging-card-summary">{{ summary }}</p>
    <p v-if="flags.length > 0" class="logging-card-flags">Logs: {{ flags.join(", ") }}</p>
    <p v-else class="muted logging-card-flags">Logs: method &amp; path only</p>
  </div>
</template>

<style scoped>
.logging-card {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #3f587c;
}

.logging-card-summary {
  margin: 0 0 6px;
}

.logging-card-flags {
  margin: 0;
  font-size: 13px;
}
</style>
