<script setup lang="ts">
import { ref, watch } from "vue";
import { NAlert, NInput, NModal, useMessage } from "naive-ui";
import { removeBlockedClient } from "@/api/blockedClient";
import { apiErrorMessage } from "@/api/http";
import type { BlockedClientMain } from "@/api/types";

const props = defineProps<{ show: boolean; row: BlockedClientMain | null }>();
const emit = defineEmits<{ "update:show": [value: boolean]; removed: [] }>();

const message = useMessage();

const reason = ref("");
const submitting = ref(false);
const serverError = ref("");

watch(
  () => props.show,
  (show) => {
    if (!show) return;
    reason.value = "";
    serverError.value = "";
  }
);

// false keeps the dialog open: it closes itself only after the server agreed.
async function confirm(): Promise<boolean> {
  const row = props.row;
  if (!row || submitting.value) return false;
  submitting.value = true;
  serverError.value = "";
  try {
    await removeBlockedClient(row.id, reason.value.trim());
    message.success(`${row.ip} unblocked`);
    emit("removed");
    emit("update:show", false);
  } catch (error) {
    serverError.value = apiErrorMessage(error, "Failed to unblock the address");
  } finally {
    submitting.value = false;
  }
  return false;
}
</script>

<template>
  <NModal
    :show="show"
    preset="dialog"
    type="warning"
    title="Unblock address"
    positive-text="Unblock"
    negative-text="Cancel"
    :loading="submitting"
    :mask-closable="!submitting"
    :on-positive-click="confirm"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <div v-if="row" class="unblock">
      <p class="unblock__text">
        Lift the block of <span class="mono">{{ row.ip }}</span>? Its requests pass again on all
        gateways within seconds; the block stays in the history.
      </p>
      <label class="unblock__field">
        <span class="unblock__label">Reason (optional)</span>
        <NInput
          v-model:value="reason"
          placeholder="e.g. false positive, the partner fixed its client"
          :disabled="submitting"
          @keydown.enter.prevent="confirm"
        />
      </label>
      <NAlert v-if="serverError" type="error" :bordered="false">{{ serverError }}</NAlert>
    </div>
  </NModal>
</template>

<style scoped>
.unblock {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.unblock__text {
  margin: 0;
}

.unblock__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.unblock__label {
  font-size: 12px;
  color: var(--c-text-3);
}
</style>
