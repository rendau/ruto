<script setup lang="ts">
import { NButton, NIcon } from "naive-ui";
import { TrashOutline } from "@vicons/ionicons5";

defineProps<{ removeLabel: string }>();
defineEmits<{ remove: [] }>();
</script>

<template>
  <div class="record">
    <div class="record__head">
      <div class="record__title"><slot name="head" /></div>
      <NButton
        class="danger-icon-button"
        type="error"
        size="small"
        secondary
        circle
        :aria-label="removeLabel"
        @click="$emit('remove')"
      >
        <NIcon :component="TrashOutline" />
      </NButton>
    </div>
    <div class="record__body"><slot /></div>
  </div>
</template>

<style scoped>
/* The head is tinted blue, not just lighter: a neutral grey reads as one more input. */
.record {
  --record-head-bg: #1f2533;
  --record-line: #323a4d;
  width: 100%;
  border: 1px solid var(--record-line);
  border-radius: 8px;
  overflow: hidden;
}

.record__head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: var(--record-head-bg);
}

.record__title {
  display: flex;
  align-items: center;
  flex: 1 1 0;
  flex-wrap: wrap;
  gap: 10px;
  min-width: 0;
}

.record__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-top: 1px solid var(--record-line);
}

.record__body:empty {
  display: none;
}
</style>
