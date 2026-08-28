<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { NButton, NIcon, NSelect, NSpin, useMessage } from "naive-ui";
import { RefreshOutline } from "@vicons/ionicons5";
import { getEndpointLogs } from "@/api/monitoring";
import { apiErrorMessage } from "@/api/http";
import { maybePrettyJson } from "@/lib/format";
import type { MonitoringLogEntry } from "@/api/types";
import SwitchField from "@/components/common/SwitchField.vue";

const props = defineProps<{
  endpointId: string;
}>();

const message = useMessage();

const RANGE_OPTIONS = [
  { label: "15m", value: 900 },
  { label: "1h", value: 3600 },
  { label: "6h", value: 21600 },
  { label: "24h", value: 86400 }
];

const range = ref(3600);
const onlyErrors = ref(false);
const loading = ref(false);
const entries = ref<MonitoringLogEntry[]>([]);
const expandedIndex = ref<number | null>(null);

function isErrorStatus(status: string): boolean {
  return /^5/.test(status) || (status !== "" && status !== "OK" && !/^[1-4]/.test(status));
}

function formatTs(tsMs: number): string {
  return new Date(tsMs).toLocaleString(undefined, {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  });
}

function toggleExpand(index: number): void {
  expandedIndex.value = expandedIndex.value === index ? null : index;
}

async function load(): Promise<void> {
  if (!props.endpointId) return;
  loading.value = true;
  expandedIndex.value = null;
  try {
    entries.value = await getEndpointLogs(props.endpointId, {
      rangeSeconds: range.value,
      onlyErrors: onlyErrors.value
    });
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load logs"));
  } finally {
    loading.value = false;
  }
}

watch([() => props.endpointId, range, onlyErrors], () => void load());
onMounted(() => void load());
</script>

<template>
  <div class="logs">
    <div class="logs__toolbar">
      <SwitchField v-model="onlyErrors" label="Only errors" />
      <div class="logs__toolbar-right">
        <NSelect v-model:value="range" size="small" :options="RANGE_OPTIONS" class="logs__range" />
        <NButton quaternary circle size="small" title="Refresh" :loading="loading" @click="load">
          <template #icon><NIcon :component="RefreshOutline" /></template>
        </NButton>
      </div>
    </div>

    <NSpin :show="loading">
      <div v-if="entries.length" class="logs__list">
        <div v-for="(entry, index) in entries" :key="index" class="logs__item">
          <button type="button" class="logs__row" @click="toggleExpand(index)">
            <span class="logs__ts">{{ formatTs(entry.ts_ms) }}</span>
            <span
              class="logs__status"
              :class="{ 'logs__status--error': isErrorStatus(entry.status) }"
            >
              {{ entry.status || "—" }}
            </span>
            <span class="logs__duration">{{ entry.duration }}</span>
            <span class="logs__message" :title="entry.error || entry.message">
              {{ entry.error || entry.message }}
            </span>
          </button>
          <pre v-if="expandedIndex === index" class="logs__raw">{{
            maybePrettyJson(entry.raw)
          }}</pre>
        </div>
      </div>
      <div v-else-if="!loading" class="logs__empty muted">No log entries for this period.</div>
    </NSpin>
  </div>
</template>

<style scoped>
.logs {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.logs__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}

.logs__toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.logs__range {
  width: 90px;
}

.logs__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.logs__row {
  display: grid;
  grid-template-columns: 118px 62px 70px 1fr;
  gap: 10px;
  align-items: center;
  width: 100%;
  padding: 6px 10px;
  border: 1px solid var(--c-border);
  border-radius: 7px;
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
  font-size: 12px;
  transition:
    border-color 0.14s ease,
    background-color 0.14s ease;
}

.logs__row:hover {
  border-color: var(--c-border-strong);
  background: var(--c-surface-2);
}

.logs__ts {
  color: var(--c-text-3);
  font-family: var(--font-mono);
  white-space: nowrap;
}

.logs__status {
  font-family: var(--font-mono);
  color: var(--c-success);
}

.logs__status--error {
  color: var(--c-error);
}

.logs__duration {
  font-family: var(--font-mono);
  color: var(--c-text-2);
  white-space: nowrap;
}

.logs__message {
  color: var(--c-text-2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.logs__raw {
  margin: 2px 0 6px;
  padding: 10px;
  border: 1px solid var(--c-border);
  border-radius: 7px;
  background: var(--c-code-bg);
  font-family: var(--font-mono);
  font-size: 11.5px;
  overflow-x: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.logs__empty {
  padding: 14px 0;
  font-size: 13px;
  text-align: center;
}

@media (max-width: 640px) {
  .logs__row {
    grid-template-columns: 1fr;
    gap: 2px;
  }
}
</style>
