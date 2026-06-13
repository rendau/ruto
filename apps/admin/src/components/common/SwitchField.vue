<script setup lang="ts">
import { NSwitch } from "naive-ui";

defineProps<{ label: string; loading?: boolean; disabled?: boolean }>();
const model = defineModel<boolean>({ required: true });
</script>

<template>
  <label class="switch-field">
    <NSwitch v-model:value="model" size="small" :loading="loading" :disabled="disabled" />
    <span class="switch-field__label">{{ label }}</span>
  </label>
</template>
