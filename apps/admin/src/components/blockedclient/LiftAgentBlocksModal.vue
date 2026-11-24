<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NAlert, NInput, NModal, NSelect, useMessage, type SelectOption } from "naive-ui";
import { removeAgentBlocks } from "@/api/blockedClient";
import { apiErrorMessage } from "@/api/http";
import type { BlockedClientMain } from "@/api/types";

const props = defineProps<{
  show: boolean;
  // the active blocks; only those with source agent are counted
  active: BlockedClientMain[];
}>();
const emit = defineEmits<{ "update:show": [value: boolean]; removed: [] }>();

const message = useMessage();

const ALL_KEYS = "";

const keyId = ref(ALL_KEYS);
const reason = ref("");
const touched = ref(false);
const submitting = ref(false);
const serverError = ref("");

watch(
  () => props.show,
  (show) => {
    if (!show) return;
    keyId.value = ALL_KEYS;
    reason.value = "";
    touched.value = false;
    serverError.value = "";
  }
);

const agentRows = computed(() => props.active.filter((row) => row.source === "agent"));

// Blocks grouped by the key that made them. The key list itself belongs to its
// owner only, so the label comes from created_by ("user / key name") of the rows.
const keyGroups = computed(() => {
  const groups = new Map<string, { label: string; count: number }>();
  for (const row of agentRows.value) {
    if (!row.api_key_id) continue;
    const group = groups.get(row.api_key_id);
    if (group) {
      group.count += 1;
    } else {
      groups.set(row.api_key_id, { label: row.created_by || row.api_key_id, count: 1 });
    }
  }
  return groups;
});

const keyOptions = computed<SelectOption[]>(() => [
  { label: `All agent keys (${agentRows.value.length})`, value: ALL_KEYS },
  ...[...keyGroups.value.entries()].map(([id, group]) => ({
    label: `${group.label} (${group.count})`,
    value: id
  }))
]);

const count = computed(() =>
  keyId.value === ALL_KEYS ? agentRows.value.length : (keyGroups.value.get(keyId.value)?.count ?? 0)
);

const reasonError = computed(() => (reason.value.trim() ? "" : "reason is required"));

// false keeps the dialog open: it closes itself only after the server agreed.
async function confirm(): Promise<boolean> {
  touched.value = true;
  if (reasonError.value || submitting.value) return false;
  submitting.value = true;
  serverError.value = "";
  try {
    const rep = await removeAgentBlocks({
      ...(keyId.value ? { api_key_id: keyId.value } : {}),
      reason: reason.value.trim()
    });
    message.success(`${rep.removed} agent block${rep.removed === 1 ? "" : "s"} lifted`);
    emit("removed");
    emit("update:show", false);
  } catch (error) {
    serverError.value = apiErrorMessage(error, "Failed to lift agent blocks");
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
    type="error"
    title="Lift agent blocks"
    :positive-text="`Lift ${count} block${count === 1 ? '' : 's'}`"
    negative-text="Cancel"
    :loading="submitting"
    :mask-closable="!submitting"
    :on-positive-click="confirm"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <div class="lift">
      <p class="lift__text">
        Lifts the active blocks made by agents (pulse, MCP agents) — the kill switch when an agent
        misbehaves. Blocks made by people stay. The lifted blocks stay in the history.
      </p>
      <label v-if="keyGroups.size > 0" class="lift__field">
        <span class="lift__label">Blocks of</span>
        <NSelect v-model:value="keyId" :options="keyOptions" :disabled="submitting" />
      </label>
      <p class="lift__count">
        <strong>{{ count }}</strong> active agent block{{ count === 1 ? "" : "s" }} will be lifted.
      </p>
      <label class="lift__field">
        <span class="lift__label">Reason</span>
        <NInput
          v-model:value="reason"
          placeholder="e.g. pulse blocked our office by mistake"
          :status="touched && reasonError ? 'error' : undefined"
          :disabled="submitting"
          @blur="touched = true"
          @keydown.enter.prevent="confirm"
        />
        <span v-if="touched && reasonError" class="field-error">{{ reasonError }}</span>
      </label>
      <NAlert v-if="serverError" type="error" :bordered="false">{{ serverError }}</NAlert>
    </div>
  </NModal>
</template>

<style scoped>
.lift {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.lift__text,
.lift__count {
  margin: 0;
}

.lift__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.lift__label {
  font-size: 12px;
  color: var(--c-text-3);
}
</style>
