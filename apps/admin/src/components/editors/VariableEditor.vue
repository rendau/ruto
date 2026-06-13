<script setup lang="ts">
import { computed } from "vue";
import { NButton, NIcon, NInput } from "naive-ui";
import { TrashOutline } from "@vicons/ionicons5";
import VariableInput from "./VariableInput.vue";
import type { Variable } from "@/api/types";

const props = withDefaults(
  defineProps<{
    modelValue: Variable[];
    availableVariables?: Variable[];
    keyPlaceholder?: string;
    valuePlaceholder?: string;
    addLabel?: string;
  }>(),
  {
    availableVariables: () => [],
    keyPlaceholder: "token",
    valuePlaceholder: "value or {{other_variable}}",
    addLabel: "Add variable"
  }
);

const emit = defineEmits<{ "update:modelValue": [value: Variable[]] }>();

const duplicateKeys = computed(() => {
  const counts = new Map<string, number>();
  for (const item of props.modelValue || []) {
    const key = (item.key || "").trim();
    if (!key) continue;
    counts.set(key, (counts.get(key) || 0) + 1);
  }
  return new Set(
    Array.from(counts.entries())
      .filter(([, count]) => count > 1)
      .map(([key]) => key)
  );
});

function updateRow(index: number, patch: Partial<Variable>): void {
  emit(
    "update:modelValue",
    (props.modelValue || []).map((item, idx) => (idx === index ? { ...item, ...patch } : item))
  );
}

function addVariable(): void {
  emit("update:modelValue", [...(props.modelValue || []), { key: "", value: "" }]);
}

function removeVariable(index: number): void {
  emit(
    "update:modelValue",
    (props.modelValue || []).filter((_, idx) => idx !== index)
  );
}
</script>

<template>
  <div class="variable-editor">
    <div v-if="modelValue.length" class="variable-editor__rows">
      <div v-for="(item, index) in modelValue" :key="index" class="variable-editor__row">
        <div class="variable-editor__key">
          <NInput
            :value="item.key"
            :placeholder="keyPlaceholder"
            @update:value="(value: string) => updateRow(index, { key: value })"
          />
          <span v-if="duplicateKeys.has((item.key || '').trim())" class="field-error">
            Duplicate key
          </span>
        </div>
        <VariableInput
          :model-value="item.value"
          :variables="availableVariables"
          :placeholder="valuePlaceholder"
          @update:model-value="(value: string) => updateRow(index, { value })"
        />
        <NButton
          class="danger-icon-button"
          type="error"
          secondary
          circle
          aria-label="Remove variable"
          @click="removeVariable(index)"
        >
          <NIcon :component="TrashOutline" />
        </NButton>
      </div>
    </div>
    <NButton size="small" dashed block @click="addVariable">+ {{ addLabel }}</NButton>
  </div>
</template>

<style scoped>
.variable-editor {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.variable-editor__rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.variable-editor__row {
  display: grid;
  grid-template-columns: minmax(120px, 0.5fr) 1fr auto;
  gap: 8px;
  align-items: start;
}

.variable-editor__key {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

@media (max-width: 560px) {
  .variable-editor__row {
    grid-template-columns: 1fr auto;
  }

  .variable-editor__key {
    grid-column: 1 / -1;
  }
}
</style>
