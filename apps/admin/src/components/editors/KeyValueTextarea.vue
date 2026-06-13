<script setup lang="ts">
import { ref, watch } from "vue";
import { NInput } from "naive-ui";
import { keyValueLinesToRecord, recordToKeyValueLines } from "@/lib/forms";

const props = withDefaults(
  defineProps<{ modelValue: Record<string, string>; placeholder?: string; rows?: number }>(),
  { placeholder: "Header-Name: value", rows: 3 }
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
  <NInput
    :value="text"
    type="textarea"
    :placeholder="placeholder"
    :autosize="{ minRows: rows, maxRows: 10 }"
    @update:value="onInput"
  />
</template>
