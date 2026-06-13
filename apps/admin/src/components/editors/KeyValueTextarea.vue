<script setup lang="ts">
import { ref, watch } from "vue";
import VariableInput from "./VariableInput.vue";
import { keyValueLinesToRecord, recordToKeyValueLines } from "@/lib/forms";
import type { Variable } from "@/api/types";

const props = withDefaults(
  defineProps<{
    modelValue: Record<string, string>;
    placeholder?: string;
    rows?: number;
    variables?: Variable[];
  }>(),
  { placeholder: "Header-Name: value", rows: 3, variables: () => [] }
);

const emit = defineEmits<{ "update:modelValue": [value: Record<string, string>] }>();

const text = ref(recordToKeyValueLines(props.modelValue));
let pushingToParent = false;

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent) {
      pushingToParent = false;
      return;
    }
    text.value = recordToKeyValueLines(value);
  },
  { deep: true }
);

function onInput(value: string): void {
  text.value = value;
  pushingToParent = true;
  emit("update:modelValue", keyValueLinesToRecord(value));
}
</script>

<template>
  <VariableInput
    type="textarea"
    :model-value="text"
    :variables="variables"
    :placeholder="placeholder"
    :autosize="{ minRows: rows, maxRows: 10 }"
    @update:model-value="onInput"
  />
</template>
