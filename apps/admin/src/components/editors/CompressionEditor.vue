<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NDynamicTags, NInputNumber, NSelect } from "naive-ui";
import { normalizeCompression } from "@/api/normalize";
import {
  COMPRESSION_ALGORITHM_OPTIONS,
  COMPRESSION_BACKEND_OPTIONS,
  COMPRESSION_RESPONSE_OPTIONS
} from "@/constants/enums";
import type {
  Compression,
  CompressionAlgorithm,
  CompressionBackend,
  CompressionResponse
} from "@/api/types";

const props = defineProps<{
  modelValue: Compression;
  // Root has nothing to inherit from: no "inherit" choices, empty = default.
  root?: boolean;
}>();

const emit = defineEmits<{ "update:modelValue": [value: Compression] }>();

const local = ref<Compression>(normalizeCompression(props.modelValue));
const pushingToParent = ref(false);

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent.value) {
      pushingToParent.value = false;
      return;
    }
    local.value = normalizeCompression(value);
  },
  { deep: true, immediate: true }
);

watch(
  local,
  (value) => {
    pushingToParent.value = true;
    emit("update:modelValue", normalizeCompression(value));
  },
  { deep: true }
);

const backendOptions = computed(() =>
  props.root ? COMPRESSION_BACKEND_OPTIONS.filter((o) => o.value !== "") : COMPRESSION_BACKEND_OPTIONS
);
const responseOptions = computed(() =>
  props.root ? COMPRESSION_RESPONSE_OPTIONS.filter((o) => o.value !== "") : COMPRESSION_RESPONSE_OPTIONS
);
const emptyHint = computed(() => (props.root ? "default" : "inherit"));

function patch(part: Partial<Compression>): void {
  local.value = { ...local.value, ...part };
}

function setContentTypes(values: string[]): void {
  patch({ content_types: values.map((v) => v.trim().toLowerCase()).filter(Boolean) });
}
</script>

<template>
  <div class="compression-editor">
    <div class="compression-editor__row">
      <label class="field">
        <span class="field__label">Ask the backend for</span>
        <NSelect
          :value="local.backend"
          :options="backendOptions"
          :placeholder="emptyHint"
          @update:value="(value: CompressionBackend) => patch({ backend: value })"
        />
      </label>
      <label class="field">
        <span class="field__label">Compress the response to the client</span>
        <NSelect
          :value="local.response"
          :options="responseOptions"
          :placeholder="emptyHint"
          @update:value="(value: CompressionResponse) => patch({ response: value })"
        />
      </label>
    </div>

    <div class="compression-editor__row">
      <label class="field field--wide">
        <span class="field__label">Algorithms, by preference (empty = {{ emptyHint }}: br, zstd, gzip)</span>
        <NSelect
          :value="local.algorithms"
          :options="COMPRESSION_ALGORITHM_OPTIONS"
          multiple
          :placeholder="emptyHint"
          @update:value="(value: CompressionAlgorithm[]) => patch({ algorithms: value })"
        />
      </label>
      <label class="field">
        <span class="field__label">Min body size, bytes (0 = {{ emptyHint }}: 1024)</span>
        <NInputNumber
          :value="local.min_size"
          :min="0"
          :step="512"
          @update:value="(value: number | null) => patch({ min_size: Math.max(0, Math.trunc(value || 0)) })"
        />
      </label>
    </div>

    <div class="field">
      <span class="field__label">Content types to compress (empty = {{ emptyHint }}: text/*, json, xml, javascript, svg)</span>
      <NDynamicTags :value="local.content_types" @update:value="setContentTypes" />
    </div>

    <p class="compression-editor__hint muted">
      The client gets the first listed algorithm its <code>Accept-Encoding</code> allows; a response
      the backend already compressed, a partial one or one marked <code>no-transform</code> is passed as is.
      <code>type/*</code> covers a whole type; <code>application/ld+json</code> matches
      <code>application/json</code>.
    </p>
  </div>
</template>

<style scoped>
.compression-editor {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  background: var(--c-surface-2);
}

.compression-editor__row {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 7px;
  min-width: 200px;
  flex: 1;
}

.field--wide {
  flex: 2;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.compression-editor__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
}

.compression-editor__hint code {
  font-family: var(--font-mono);
  color: var(--c-text-2);
}

@media (max-width: 560px) {
  .compression-editor__row {
    flex-direction: column;
  }
}
</style>
