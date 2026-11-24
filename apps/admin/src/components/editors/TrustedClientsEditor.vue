<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NButton, NIcon, NInput } from "naive-ui";
import { AddOutline, TrashOutline } from "@vicons/ionicons5";
import { emptyRootTrustedClient, normalizeRootTrustedClients } from "@/api/normalize";
import { trustedClientError } from "@/lib/ip";
import type { RootTrustedClient } from "@/api/types";

const props = defineProps<{ modelValue: RootTrustedClient[] }>();
const emit = defineEmits<{ "update:modelValue": [value: RootTrustedClient[]] }>();

const local = ref<RootTrustedClient[]>(normalizeRootTrustedClients(props.modelValue));
const rowKeys = ref<string[]>([]);
const pushingToParent = ref(false);
let nextRowKey = 0;

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent.value) {
      pushingToParent.value = false;
      return;
    }
    local.value = normalizeRootTrustedClients(value);
    syncRowKeys();
  },
  { deep: true, immediate: true }
);

watch(
  local,
  (value) => {
    pushingToParent.value = true;
    emit("update:modelValue", normalizeRootTrustedClients(value));
  },
  { deep: true }
);

const rowErrors = computed(() => local.value.map(trustedClientError));

function syncRowKeys(): void {
  const count = local.value.length;
  if (rowKeys.value.length < count) {
    rowKeys.value = [
      ...rowKeys.value,
      ...Array.from({ length: count - rowKeys.value.length }, () => {
        nextRowKey += 1;
        return `trusted-client-${nextRowKey}`;
      })
    ];
  } else if (rowKeys.value.length > count) {
    rowKeys.value = rowKeys.value.slice(0, count);
  }
}

function addRow(): void {
  local.value = [...local.value, emptyRootTrustedClient()];
  syncRowKeys();
}

function removeRow(rowIndex: number): void {
  local.value = local.value.filter((_, idx) => idx !== rowIndex);
  rowKeys.value = rowKeys.value.filter((_, idx) => idx !== rowIndex);
}

function updateRow(rowIndex: number, part: Partial<RootTrustedClient>): void {
  local.value = local.value.map((item, idx) => (idx === rowIndex ? { ...item, ...part } : item));
}
</script>

<template>
  <div class="trusted-editor">
    <p v-if="local.length === 0" class="trusted-editor__empty muted">
      No trusted clients — every client is subject to rate limits and blocks.
    </p>

    <div v-else class="trusted-editor__list">
      <div class="trusted-row trusted-row--head" aria-hidden="true">
        <span class="field__label">Name</span>
        <span class="field__label">Address or network</span>
        <span />
      </div>
      <template v-for="(item, rowIndex) in local" :key="rowKeys[rowIndex] || rowIndex">
        <div class="trusted-row">
          <NInput
            :value="item.name"
            placeholder="e.g. 1C"
            :input-props="{ 'aria-label': `Trusted client ${rowIndex + 1} name` }"
            @update:value="(value: string) => updateRow(rowIndex, { name: value })"
          />
          <NInput
            :value="item.ip"
            placeholder="192.168.62.3 or 10.0.0.0/8"
            :status="rowErrors[rowIndex] ? 'error' : undefined"
            :input-props="{ 'aria-label': `Trusted client ${rowIndex + 1} address` }"
            @update:value="(value: string) => updateRow(rowIndex, { ip: value })"
          />
          <NButton
            class="danger-icon-button"
            type="error"
            secondary
            circle
            aria-label="Remove trusted client"
            @click="removeRow(rowIndex)"
          >
            <NIcon :component="TrashOutline" />
          </NButton>
        </div>
        <p v-if="rowErrors[rowIndex]" class="trusted-row__error">{{ rowErrors[rowIndex] }}</p>
      </template>
    </div>

    <NButton dashed block @click="addRow">
      <template #icon><NIcon :component="AddOutline" /></template>
      Add trusted client
    </NButton>

    <p class="trusted-editor__hint muted">
      An address (<code>192.168.62.3</code>, <code>2001:db8::1</code>) or a network in CIDR notation
      (<code>10.0.0.0/8</code>); a single address is stored as <code>/32</code> (IPv6:
      <code>/128</code>). Requests from these clients are never rate-limited, and they can never be
      blocked: the block list refuses an address or network that overlaps one of them.
    </p>
    <p class="trusted-editor__hint muted">
      The list is part of the configuration: it reaches the gateways with a Deploy, and a change
      proposed by an MCP agent goes through a change request a person approves — so an agent can't
      lift limits by itself.
    </p>
  </div>
</template>

<style scoped>
.trusted-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  background: var(--c-surface-2);
}

.trusted-editor__empty {
  margin: 0;
  font-size: 13px;
}

.trusted-editor__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.trusted-row {
  display: grid;
  grid-template-columns: minmax(120px, 0.6fr) minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
}

.trusted-row--head {
  margin-bottom: -2px;
}

.trusted-row__error {
  margin: -2px 0 0;
  font-size: 12px;
  color: var(--c-error);
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.trusted-editor__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
}

.trusted-editor__hint code {
  font-family: var(--font-mono);
  color: var(--c-text-2);
}

@media (max-width: 560px) {
  .trusted-row {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .trusted-row > :first-child {
    grid-column: 1 / -1;
  }

  .trusted-row--head {
    display: none;
  }
}
</style>
