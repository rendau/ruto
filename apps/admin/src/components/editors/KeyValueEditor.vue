<script setup lang="ts">
import { ref, watch } from "vue";
import { NButton, NIcon, NInput } from "naive-ui";
import { TrashOutline } from "@vicons/ionicons5";
import VariableInput from "./VariableInput.vue";
import type { Variable } from "@/api/types";

const props = withDefaults(
  defineProps<{
    modelValue: Record<string, string>;
    availableVariables?: Variable[];
    keyPlaceholder?: string;
    valuePlaceholder?: string;
    addLabel?: string;
  }>(),
  {
    availableVariables: () => [],
    keyPlaceholder: "Name",
    valuePlaceholder: "value or {{variable}}",
    addLabel: "Add"
  }
);

const emit = defineEmits<{ "update:modelValue": [value: Record<string, string>] }>();

interface Row {
  key: string;
  value: string;
}

const rows = ref<Row[]>(toRows(props.modelValue));
let pushingToParent = false;

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent) {
      pushingToParent = false;
      return;
    }
    rows.value = toRows(value);
  },
  { deep: true }
);

function toRows(record: Record<string, string>): Row[] {
  return Object.entries(record || {}).map(([key, value]) => ({ key, value }));
}

function toRecord(list: Row[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (const { key, value } of list) {
    const trimmed = key.trim();
    if (trimmed) {
      result[trimmed] = value;
    }
  }
  return result;
}

function emitRows(): void {
  pushingToParent = true;
  emit("update:modelValue", toRecord(rows.value));
}

function updateRow(index: number, patch: Partial<Row>): void {
  rows.value = rows.value.map((row, idx) => (idx === index ? { ...row, ...patch } : row));
  emitRows();
}

function addRow(): void {
  rows.value = [...rows.value, { key: "", value: "" }];
}

function removeRow(index: number): void {
  rows.value = rows.value.filter((_, idx) => idx !== index);
  emitRows();
}
</script>

<template>
  <div class="kv-editor">
    <div v-if="rows.length" class="kv-editor__rows">
      <div v-for="(row, index) in rows" :key="index" class="kv-editor__row">
        <NInput
          :value="row.key"
          :placeholder="keyPlaceholder"
          @update:value="(value: string) => updateRow(index, { key: value })"
        />
        <VariableInput
          :model-value="row.value"
          :variables="availableVariables"
          :placeholder="valuePlaceholder"
          @update:model-value="(value: string) => updateRow(index, { value })"
        />
        <NButton
          class="danger-icon-button"
          type="error"
          secondary
          circle
          aria-label="Remove"
          @click="removeRow(index)"
        >
          <NIcon :component="TrashOutline" />
        </NButton>
      </div>
    </div>
    <NButton size="small" dashed block @click="addRow">+ {{ addLabel }}</NButton>
  </div>
</template>

<style scoped>
.kv-editor {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.kv-editor__rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.kv-editor__row {
  display: grid;
  grid-template-columns: minmax(120px, 0.5fr) 1fr auto;
  gap: 8px;
  align-items: start;
}

@media (max-width: 560px) {
  .kv-editor__row {
    grid-template-columns: 1fr auto;
  }
}
</style>
