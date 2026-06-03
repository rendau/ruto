<script setup lang="ts">
import { computed } from "vue";
import type { Variable } from "../types/api";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    variables?: Variable[];
    type?: "text" | "textarea" | "password";
    placeholder?: string;
    rows?: number;
    required?: boolean;
    autosize?: boolean | { minRows?: number; maxRows?: number };
  }>(),
  {
    variables: () => [],
    type: "text",
    placeholder: "",
    rows: 3,
    required: false,
    autosize: false
  }
);

const emit = defineEmits<{
  (event: "update:modelValue", value: string): void;
}>();

const uniqueVariables = computed(() => {
  const seen = new Set<string>();
  return props.variables
    .map((item) => ({ key: (item.key || "").trim(), value: item.value || "" }))
    .filter((item) => {
      if (!item.key || seen.has(item.key)) {
        return false;
      }
      seen.add(item.key);
      return true;
    });
});

const trigger = computed(() => {
  const value = props.modelValue || "";
  const start = value.lastIndexOf("{{");
  if (start < 0) {
    return { active: false, query: "", start: -1 };
  }
  const tail = value.slice(start + 2);
  if (tail.includes("}}") || /[\s{}]/.test(tail)) {
    return { active: false, query: "", start };
  }
  return { active: true, query: tail.trim().toLowerCase(), start };
});

const suggestions = computed(() => {
  if (!trigger.value.active) {
    return [];
  }
  const query = trigger.value.query;
  return uniqueVariables.value
    .filter((item) => !query || item.key.toLowerCase().includes(query))
    .slice(0, 8);
});

function updateValue(value: string) {
  emit("update:modelValue", value);
}

function insertVariable(key: string) {
  const value = props.modelValue || "";
  const start = trigger.value.start;
  if (start < 0) {
    emit("update:modelValue", `${value}{{${key}}}`);
    return;
  }
  emit("update:modelValue", `${value.slice(0, start)}{{${key}}}`);
}
</script>

<template>
  <div class="variable-input">
    <n-input
      :value="modelValue"
      :type="type"
      :rows="rows"
      :placeholder="placeholder"
      :required="required"
      :autosize="autosize"
      @update:value="updateValue"
    />
    <div v-if="suggestions.length > 0" class="variable-suggest" role="listbox">
      <button
        v-for="item in suggestions"
        :key="item.key"
        class="variable-suggest-item"
        type="button"
        @mousedown.prevent="insertVariable(item.key)"
      >
        <span class="variable-suggest-key">{{ item.key }}</span>
        <span class="variable-suggest-value">{{ item.value }}</span>
      </button>
    </div>
  </div>
</template>

