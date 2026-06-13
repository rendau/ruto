<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import getCaretCoordinates from "textarea-caret";
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

const rootRef = ref<HTMLElement | null>(null);
const caretIndex = ref(0);
const activeIndex = ref(0);
const popupPosition = ref({ top: 0, left: 0 });
const hasFocus = ref(false);
let pendingCaretIndex: number | null = null;

type InputElement = HTMLInputElement | HTMLTextAreaElement;

function getInputElement(): InputElement | null {
  if (!rootRef.value) {
    return null;
  }
  return rootRef.value.querySelector("textarea, input");
}

function updateCaretIndex() {
  const el = getInputElement();
  if (!el) {
    caretIndex.value = (props.modelValue || "").length;
    return;
  }
  caretIndex.value = el.selectionStart ?? (props.modelValue || "").length;
}

function updatePopupPosition() {
  if (!trigger.value.active) {
    return;
  }
  const root = rootRef.value;
  const el = getInputElement();
  if (!root || !el) {
    return;
  }
  const index = el.selectionStart ?? 0;
  const coords = getCaretCoordinates(el, index);
  const rootRect = root.getBoundingClientRect();
  const inputRect = el.getBoundingClientRect();
  popupPosition.value = {
    top: inputRect.top - rootRect.top + coords.top + coords.height + 8,
    left: inputRect.left - rootRect.left + coords.left
  };
}

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
  nextTick(() => {
    updateCaretIndex();
    updatePopupPosition();
  });
}

function insertVariable(key: string) {
  const value = props.modelValue || "";
  const start = trigger.value.start;
  const cursor = Math.max(0, Math.min(caretIndex.value, value.length));
  const nextValue = start < 0
    ? `${value}{{${key}}}`
    : `${value.slice(0, start)}{{${key}}}${value.slice(cursor)}`;
  const nextCaret = start < 0 ? nextValue.length : start + key.length + 4;
  pendingCaretIndex = nextCaret;
  emit("update:modelValue", nextValue);
}

function onKeydown(event: KeyboardEvent) {
  if (!suggestions.value.length) {
    return;
  }
  if (event.key === "ArrowDown") {
    event.preventDefault();
    activeIndex.value = (activeIndex.value + 1) % suggestions.value.length;
    return;
  }
  if (event.key === "ArrowUp") {
    event.preventDefault();
    activeIndex.value = (activeIndex.value - 1 + suggestions.value.length) % suggestions.value.length;
    return;
  }
  if (event.key === "Enter" || event.key === "Tab") {
    event.preventDefault();
    insertVariable(suggestions.value[activeIndex.value].key);
    return;
  }
  if (event.key === "Escape") {
    event.preventDefault();
    activeIndex.value = 0;
  }
}

function onFocus() {
  hasFocus.value = true;
  updateCaretIndex();
  updatePopupPosition();
}

function onBlur() {
  hasFocus.value = false;
}

function onCursorEvent() {
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
      if (!el) {
        return;
      }
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
    <n-input
      :value="modelValue"
      :type="type"
      :rows="rows"
      :placeholder="placeholder"
      :required="required"
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
        class="variable-suggest-item"
        :class="{ 'variable-suggest-item--active': index === activeIndex }"
        type="button"
        @mouseenter="activeIndex = index"
        @mousedown.prevent="insertVariable(item.key)"
      >
        <span class="variable-suggest-key">{{ item.key }}</span>
        <span class="variable-suggest-value">{{ item.value }}</span>
      </button>
    </div>
  </div>
</template>
