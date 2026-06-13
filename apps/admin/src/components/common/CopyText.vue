<script setup lang="ts">
import { NButton, NIcon } from "naive-ui";
import { CopyOutline } from "@vicons/ionicons5";
import { useClipboard } from "@/composables/useClipboard";

const props = withDefaults(
  defineProps<{ value: string; label?: string; placeholder?: string; mono?: boolean }>(),
  { placeholder: "—", mono: true }
);

const { copy } = useClipboard();
</script>

<template>
  <span class="copy-text">
    <span class="copy-text__value" :class="{ mono }" :title="value">
      {{ value || placeholder }}
    </span>
    <NButton
      v-if="value"
      class="copy-text__btn"
      quaternary
      circle
      size="tiny"
      :aria-label="label ? `Copy ${label}` : 'Copy'"
      @click="copy(value, label ? `${label} copied` : 'Copied')"
    >
      <NIcon :component="CopyOutline" />
    </NButton>
  </span>
</template>

<style scoped>
.copy-text {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  max-width: 100%;
}

.copy-text__value {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--c-text);
}

.copy-text__btn {
  flex-shrink: 0;
  opacity: 0.65;
}

.copy-text__btn:hover {
  opacity: 1;
}
</style>
