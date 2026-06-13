<script setup lang="ts">
import { ref, watch } from "vue";
import { NCheckbox, NInputNumber, NSelect } from "naive-ui";
import { normalizeLogging } from "@/api/normalize";
import { LOGGING_LEVEL_OPTIONS, LOGGING_MODE_OPTIONS } from "@/constants/enums";
import type { Logging, LoggingLevel, LoggingMode } from "@/api/types";

const props = defineProps<{
  modelValue: Logging;
  hideMode?: boolean;
}>();

const emit = defineEmits<{ "update:modelValue": [value: Logging] }>();

const local = ref<Logging>(normalizeLogging(props.modelValue));
const pushingToParent = ref(false);

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

function patch(part: Partial<Logging>): void {
  local.value = { ...local.value, ...part };
}

function setLimit(field: "req_body_limit" | "resp_body_limit", value: number | null): void {
  patch({ [field]: Math.max(0, Math.trunc(value || 0)) } as Partial<Logging>);
}
</script>

<template>
  <div class="logging-editor">
    <div class="logging-editor__head">
      <label v-if="!hideMode" class="field">
        <span class="field__label">Mode</span>
        <NSelect
          :value="local.mode"
          :options="LOGGING_MODE_OPTIONS"
          @update:value="(value: LoggingMode) => patch({ mode: value })"
        />
      </label>
      <label class="field">
        <span class="field__label">Level</span>
        <NSelect
          :value="local.level"
          :options="LOGGING_LEVEL_OPTIONS"
          @update:value="(value: LoggingLevel) => patch({ level: value })"
        />
      </label>
    </div>

    <div class="logging-editor__flags">
      <span class="field__label">Capture (method &amp; path are always logged)</span>
      <div class="logging-editor__checks">
        <NCheckbox
          :checked="local.headers"
          @update:checked="(value: boolean) => patch({ headers: value })"
        >
          Headers
        </NCheckbox>
        <NCheckbox
          :checked="local.query_params"
          @update:checked="(value: boolean) => patch({ query_params: value })"
        >
          Query params
        </NCheckbox>
        <NCheckbox
          :checked="local.req_body"
          @update:checked="(value: boolean) => patch({ req_body: value })"
        >
          Request body
        </NCheckbox>
        <NCheckbox
          :checked="local.resp_body"
          @update:checked="(value: boolean) => patch({ resp_body: value })"
        >
          Response body
        </NCheckbox>
      </div>
    </div>

    <div v-if="local.req_body || local.resp_body" class="logging-editor__limits">
      <label v-if="local.req_body" class="field">
        <span class="field__label">Request body limit (bytes, 0 = default)</span>
        <NInputNumber
          :value="local.req_body_limit"
          :min="0"
          :step="512"
          @update:value="(value: number | null) => setLimit('req_body_limit', value)"
        />
      </label>
      <label v-if="local.resp_body" class="field">
        <span class="field__label">Response body limit (bytes, 0 = default)</span>
        <NInputNumber
          :value="local.resp_body_limit"
          :min="0"
          :step="512"
          @update:value="(value: number | null) => setLimit('resp_body_limit', value)"
        />
      </label>
    </div>

    <p class="logging-editor__hint muted">
      <code>authorization</code> and <code>api-key</code> values are always masked in logged headers,
      query params and gRPC metadata. Bodies beyond the limit are truncated.
    </p>
  </div>
</template>

<style scoped>
.logging-editor {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  background: var(--c-surface-2);
}

.logging-editor__head {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--c-border);
}

.logging-editor__flags,
.field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.field {
  min-width: 200px;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.logging-editor__checks {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.logging-editor__limits {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.logging-editor__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
}

.logging-editor__hint code {
  font-family: var(--font-mono);
  color: var(--c-text-2);
}

@media (max-width: 560px) {
  .logging-editor__head {
    flex-direction: column;
  }
}
</style>
