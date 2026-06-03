<script setup lang="ts">
import { computed } from "vue";
import { TrashOutline } from "@vicons/ionicons5";
import type { Variable } from "../types/api";
import VariableInput from "./VariableInput.vue";

const props = withDefaults(
  defineProps<{
    modelValue: Variable[];
    availableVariables?: Variable[];
  }>(),
  {
    availableVariables: () => []
  }
);

const emit = defineEmits<{
  (event: "update:modelValue", value: Variable[]): void;
}>();

const duplicateKeys = computed(() => {
  const counts = new Map<string, number>();
  for (const item of props.modelValue || []) {
    const key = (item.key || "").trim();
    if (!key) {
      continue;
    }
    counts.set(key, (counts.get(key) || 0) + 1);
  }
  return new Set(Array.from(counts.entries()).filter(([, count]) => count > 1).map(([key]) => key));
});

function updateRow(index: number, patch: Partial<Variable>) {
  emit(
    "update:modelValue",
    (props.modelValue || []).map((item, idx) => (idx === index ? { ...item, ...patch } : item))
  );
}

function addVariable() {
  emit("update:modelValue", [...(props.modelValue || []), { key: "", value: "" }]);
}

function removeVariable(index: number) {
  emit("update:modelValue", (props.modelValue || []).filter((_, idx) => idx !== index));
}
</script>

<template>
  <div class="variable-editor">
    <div class="variable-editor-head">
      <span>Key</span>
      <span>Value</span>
      <span />
    </div>
    <div v-for="(item, index) in modelValue" :key="index" class="variable-editor-row">
      <div class="variable-key-cell">
        <n-input :value="item.key" placeholder="token" @update:value="(value: string) => updateRow(index, { key: value })" />
        <span v-if="duplicateKeys.has((item.key || '').trim())" class="field-error">Duplicate key</span>
      </div>
      <VariableInput
        :model-value="item.value"
        :variables="availableVariables"
        placeholder="secret or {{other_variable}}"
        @update:model-value="(value: string) => updateRow(index, { value })"
      />
      <n-button
        class="danger-icon-button"
        type="error"
        secondary
        circle
        aria-label="Remove variable"
        title="Remove variable"
        @click="removeVariable(index)"
      >
        <n-icon :component="TrashOutline" />
      </n-button>
    </div>
    <n-button size="small" secondary @click="addVariable">+ Variable</n-button>
  </div>
</template>

