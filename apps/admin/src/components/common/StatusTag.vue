<script setup lang="ts">
import { NTag } from "naive-ui";

withDefaults(
  defineProps<{ active: boolean; activeText?: string; inactiveText?: string }>(),
  { activeText: "Active", inactiveText: "Inactive" }
);
</script>

<template>
  <NTag :type="active ? 'success' : 'default'" size="small" :bordered="false" round>
    <span class="dot" :class="{ 'dot--on': active }" />
    {{ active ? activeText : inactiveText }}
  </NTag>
</template>

<style scoped>
.dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-right: 5px;
  border-radius: 999px;
  background: var(--c-text-3);
  vertical-align: middle;
}

.dot--on {
  background: var(--c-success);
}
</style>
