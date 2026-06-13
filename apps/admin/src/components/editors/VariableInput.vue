<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { NInput } from "naive-ui";
import getCaretCoordinates from "textarea-caret";
import type { Variable } from "@/api/types";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    variables?: Variable[];
    type?: "text" | "textarea" | "password";
    placeholder?: string;
    rows?: number;
    autosize?: boolean | { minRows?: number; maxRows?: number };
  }>(),
  {
    variables: () => [],
    type: "text",
    placeholder: "",
    rows: 3,
    autosize: false
  }
);

const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const rootRef = ref<HTMLElement | null>(null);
const caretIndex = ref(0);
const activeIndex = ref(0);
const popupPosition = ref({ top: 0, left: 0 });
const hasFocus = ref(false);
let pendingCaretIndex: number | null = null;

type InputElement = HTMLInputElement | HTMLTextAreaElement;

function getInputElement(): InputElement | null {
  return rootRef.value?.querySelector("textarea, input") ?? null;
}

function updateCaretIndex(): void {
  const el = getInputElement();
  caretIndex.value = el?.selectionStart ?? (props.modelValue || "").length;
}

const uniqueVariables = computed(() => {
  const seen = new Set<string>();
  return props.variables
    .map((item) => ({ key: (item.key || "").trim(), value: item.value || "" }))
    .filter((item) => {
      if (!item.key || seen.has(item.key)) return false;
      seen.add(item.key);
      return true;
    });
});

const trigger = computed(() => {
  const value = props.modelValue || "";
  const cursor = Math.max(0, Math.min(caretIndex.value, value.length));
  const beforeCursor = value.slice(0, cursor);
  const start = beforeCursor.lastIndexOf("{{");
  if (start < 0) {
    return { active: false, query: "", start: -1 };
  }
  const tail = beforeCursor.slice(start + 2);
  if (tail.includes("}}") || /[\s{}]/.test(tail)) {
    return { active: false, query: "", start };
  }
  return { active: true, query: tail.trim().toLowerCase(), start };
});

const suggestions = computed(() => {
  if (!trigger.value.active) return [];
  const query = trigger.value.query;
  return uniqueVariables.value
    .filter((item) => !query || item.key.toLowerCase().includes(query))
    .slice(0, 8);
});

function updatePopupPosition(): void {
  if (!trigger.value.active) return;
  const root = rootRef.value;
  const el = getInputElement();
  if (!root || !el) return;
  const index = el.selectionStart ?? 0;
  const coords = getCaretCoordinates(el, index);
  const rootRect = root.getBoundingClientRect();
  const inputRect = el.getBoundingClientRect();
  popupPosition.value = {
    top: inputRect.top - rootRect.top + coords.top + coords.height + 8,
    left: inputRect.left - rootRect.left + coords.left
  };
}

function updateValue(value: string): void {
  emit("update:modelValue", value);
  nextTick(() => {
    updateCaretIndex();
    updatePopupPosition();
  });
}

function insertVariable(key: string): void {
  const value = props.modelValue || "";
  const start = trigger.value.start;
  const cursor = Math.max(0, Math.min(caretIndex.value, value.length));
  const nextValue =
    start < 0 ? `${value}{{${key}}}` : `${value.slice(0, start)}{{${key}}}${value.slice(cursor)}`;
  pendingCaretIndex = start < 0 ? nextValue.length : start + key.length + 4;
  emit("update:modelValue", nextValue);
}

function onKeydown(event: KeyboardEvent): void {
  if (!suggestions.value.length) return;
  if (event.key === "ArrowDown") {
    event.preventDefault();
    activeIndex.value = (activeIndex.value + 1) % suggestions.value.length;
  } else if (event.key === "ArrowUp") {
    event.preventDefault();
    activeIndex.value = (activeIndex.value - 1 + suggestions.value.length) % suggestions.value.length;
  } else if (event.key === "Enter" || event.key === "Tab") {
    const selected = suggestions.value[activeIndex.value];
    if (selected) {
      event.preventDefault();
      insertVariable(selected.key);
    }
  } else if (event.key === "Escape") {
    event.preventDefault();
    activeIndex.value = 0;
  }
}

function onFocus(): void {
  hasFocus.value = true;
  updateCaretIndex();
  updatePopupPosition();
}

function onBlur(): void {
  hasFocus.value = false;
}

function onCursorEvent(): void {
  updateCaretIndex();
  updatePopupPosition();
}

watch(suggestions, (list) => {
  activeIndex.value = list.length ? Math.min(activeIndex.value, list.length - 1) : 0;
  nextTick(updatePopupPosition);
});

watch(
  () => props.modelValue,
  () => {
    nextTick(() => {
      const el = getInputElement();
      if (!el) return;
      if (pendingCaretIndex !== null) {
        const index = Math.min(pendingCaretIndex, (props.modelValue || "").length);
        el.setSelectionRange(index, index);
        caretIndex.value = index;
        pendingCaretIndex = null;
      } else {
        updateCaretIndex();
      }
      updatePopupPosition();
    });
  }
);

onMounted(() => {
  window.addEventListener("resize", updatePopupPosition);
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", updatePopupPosition);
});
</script>

<template>
  <div ref="rootRef" class="variable-input">
    <NInput
      :value="modelValue"
      :type="type"
      :rows="rows"
      :placeholder="placeholder"
      :autosize="autosize"
      @update:value="updateValue"
      @keydown="onKeydown"
      @keyup="onCursorEvent"
      @click="onCursorEvent"
      @focus="onFocus"
      @blur="onBlur"
    />
    <div
      v-if="hasFocus && suggestions.length > 0"
      class="variable-suggest"
      role="listbox"
      :style="{ top: `${popupPosition.top}px`, left: `${popupPosition.left}px` }"
    >
      <button
        v-for="(item, index) in suggestions"
        :key="item.key"
        class="variable-suggest__item"
        :class="{ 'variable-suggest__item--active': index === activeIndex }"
        type="button"
        @mouseenter="activeIndex = index"
        @mousedown.prevent="insertVariable(item.key)"
      >
        <span class="variable-suggest__key mono">{{ item.key }}</span>
        <span class="variable-suggest__value">{{ item.value }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.variable-input {
  position: relative;
  width: 100%;
}

.variable-suggest {
  position: absolute;
  z-index: 60;
  min-width: 200px;
  max-width: min(360px, calc(100vw - 24px));
  max-height: 260px;
  overflow-y: auto;
  padding: 4px;
  border: 1px solid var(--c-border-strong);
  border-radius: 9px;
  background: var(--c-surface-2);
  box-shadow: var(--shadow-md);
}

.variable-suggest__item {
  display: flex;
  flex-direction: column;
  gap: 1px;
  width: 100%;
  padding: 6px 9px;
  border: none;
  border-radius: 6px;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.variable-suggest__item--active {
  background: var(--c-primary-soft);
}

.variable-suggest__key {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--c-text);
}

.variable-suggest__value {
  font-size: 11.5px;
  color: var(--c-text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
