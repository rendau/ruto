<script setup lang="ts">
import { useDialog, useLoadingBar, useMessage, useNotification } from "naive-ui";
import { registerFeedback } from "@/lib/feedback";

// Captures the provider-scoped feedback APIs into a module singleton so the
// API layer and other non-component code can show toasts/dialogs.
registerFeedback({
  message: useMessage(),
  dialog: useDialog(),
  notification: useNotification(),
  loadingBar: useLoadingBar()
});
</script>

<template>
  <slot />
</template>
