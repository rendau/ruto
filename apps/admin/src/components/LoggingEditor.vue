<script setup lang="ts">
import { ref, watch } from "vue";
import { normalizeLogging } from "../lib/forms";
import type { Logging } from "../types/api";

const props = defineProps<{
  modelValue: Logging;
  hideMode?: boolean;
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: Logging): void;
}>();

const local = ref<Logging>(normalizeLogging(props.modelValue));
const pushingToParent = ref(false);

const levelOptions = [
  { label: "inherit", value: "" },
  { label: "all", value: "all" },
  { label: "error", value: "error" },
  { label: "don't log", value: "none" }
];

const modeOptions = [
  { label: "extend", value: "extend" },
  { label: "replace", value: "replace" }
];

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent.value) {
      pushingToParent.value = false;
      return;
    }
    local.value = normalizeLogging(value);
  },
  { deep: true, immediate: true }
);

watch(
  local,
  (value) => {
    pushingToParent.value = true;
    emit("update:modelValue", normalizeLogging(value));
  },
  { deep: true }
);

function patch(part: Partial<Logging>) {
  local.value = { ...local.value, ...part };
}

function setLimit(field: "req_body_limit" | "resp_body_limit", value: number | null) {
  patch({ [field]: Math.max(0, Math.trunc(value || 0)) } as Partial<Logging>);
}
</script>

<template>
  <div class="logging-editor">
    <div class="logging-head">
      <label v-if="!hideMode" class="logging-field">
        <span>Mode</span>
        <n-select
          :value="local.mode"
          :options="modeOptions"
          @update:value="(value: string) => patch({ mode: value === 'replace' ? 'replace' : 'extend' })"
        />
      </label>
      <label class="logging-field">
        <span>Level</span>
        <n-select
          :value="local.level"
          :options="levelOptions"
          @update:value="(value: string) => patch({ level: value || '' })"
        />
      </label>
    </div>

    <div class="logging-flags">
      <span class="logging-flags-label">Log (method &amp; path are always logged):</span>
      <n-checkbox :checked="local.headers" @update:checked="(value: boolean) => patch({ headers: value })">Headers</n-checkbox>
      <n-checkbox :checked="local.query_params" @update:checked="(value: boolean) => patch({ query_params: value })">Query params</n-checkbox>
      <n-checkbox :checked="local.req_body" @update:checked="(value: boolean) => patch({ req_body: value })">Request body</n-checkbox>
      <n-checkbox :checked="local.resp_body" @update:checked="(value: boolean) => patch({ resp_body: value })">Response body</n-checkbox>
    </div>

    <div v-if="local.req_body || local.resp_body" class="logging-limits">
      <label v-if="local.req_body" class="logging-field">
        <span>Request body limit (bytes, 0 = default)</span>
        <n-input-number
          :value="local.req_body_limit"
          :min="0"
          :step="512"
          @update:value="(value: number | null) => setLimit('req_body_limit', value)"
        />
      </label>
      <label v-if="local.resp_body" class="logging-field">
        <span>Response body limit (bytes, 0 = default)</span>
        <n-input-number
          :value="local.resp_body_limit"
          :min="0"
          :step="512"
          @update:value="(value: number | null) => setLimit('resp_body_limit', value)"
        />
      </label>
    </div>

    <p class="muted logging-hint">
      <code>authorization</code> and <code>api-key</code> values are always masked in logged headers, query params and gRPC metadata.
      Bodies beyond the limit are truncated.
    </p>
  </div>
</template>

<style scoped>
.logging-editor {
  border: 1px solid #425c7f;
  border-radius: 8px;
  background: #1e2d44;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.logging-head {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid #2c405d;
}

.logging-limits {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.logging-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 200px;
}

.logging-field > span {
  font-size: 12px;
  color: #9fb3d1;
}

.logging-flags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 14px;
}

.logging-flags-label {
  font-size: 12px;
  color: #9fb3d1;
}

.logging-hint {
  margin: 0;
  font-size: 12px;
}

@media (max-width: 560px) {
  .logging-editor {
    padding-left: 8px;
    padding-right: 8px;
  }

  .logging-head {
    flex-direction: column;
  }

  .logging-field {
    min-width: 0;
  }
}
</style>
