<script setup lang="ts">
import { NButton, NIcon } from "naive-ui";
import { CopyOutline } from "@vicons/ionicons5";
import { useClipboard } from "@/composables/useClipboard";

defineProps<{ content: string; maxHeight?: string }>();

const { copy } = useClipboard();
</script>

<template>
  <div class="json-block">
    <NButton
      v-if="content"
      class="json-block__copy"
      size="tiny"
      secondary
      @click="copy(content)"
    >
      <template #icon><NIcon :component="CopyOutline" /></template>
      Copy
    </NButton>
    <pre class="json-block__pre mono" :style="{ maxHeight: maxHeight || '360px' }">{{ content }}</pre>
  </div>
</template>

<style scoped>
.json-block {
  position: relative;
}

.json-block__copy {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 1;
}

.json-block__pre {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid var(--c-border);
  border-radius: 9px;
  background: var(--c-code-bg);
  color: #d4e4fa;
  font-size: 12.5px;
  line-height: 1.55;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
