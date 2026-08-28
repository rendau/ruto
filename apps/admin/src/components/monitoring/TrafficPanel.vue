<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { NButton, NIcon, NSelect, NSpin, useMessage } from "naive-ui";
import { RefreshOutline } from "@vicons/ionicons5";
import { getAppMetrics, getEndpointMetrics } from "@/api/monitoring";
import { apiErrorMessage } from "@/api/http";
import type { MonitoringSeries } from "@/api/types";
import SparklineChart from "@/components/monitoring/SparklineChart.vue";

const props = defineProps<{
  kind: "app" | "endpoint";
  id: string;
}>();

const message = useMessage();

const RANGE_OPTIONS = [
  { label: "1h", value: 3600 },
  { label: "6h", value: 21600 },
  { label: "24h", value: 86400 },
  { label: "7d", value: 604800 }
];

const range = ref(3600);
const loading = ref(false);
const series = ref<MonitoringSeries | null>(null);

const points = computed(() => series.value?.points ?? []);

const rpsPoints = computed(() => points.value.map((p) => ({ ts: p.ts, value: p.rps })));
const durationPoints = computed(() =>
  points.value.map((p) => ({ ts: p.ts, value: p.duration_avg_seconds }))
);
const errorPoints = computed(() => points.value.map((p) => ({ ts: p.ts, value: p.error_rate })));

const lastPoint = computed(() => points.value[points.value.length - 1] ?? null);

function formatRps(value: number): string {
  if (value >= 100) return value.toFixed(0);
  if (value >= 10) return value.toFixed(1);
  return value.toFixed(2);
}

function formatDurationSeconds(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0 ms";
  const ms = value * 1000;
  if (ms < 1) return "<1 ms";
  if (ms < 100) return `${ms.toFixed(1)} ms`;
  if (ms < 1000) return `${ms.toFixed(0)} ms`;
  return `${value.toFixed(2)} s`;
}

function formatErrorRate(value: number): string {
  return `${(value * 100).toFixed(1)}%`;
}

const formatTs = computed(() => {
  const withDate = range.value > 21600;
  return (ts: number) =>
    new Date(ts * 1000).toLocaleString(undefined, {
      ...(withDate ? { day: "2-digit", month: "2-digit" } : {}),
      hour: "2-digit",
      minute: "2-digit"
    });
});

async function load(): Promise<void> {
  if (!props.id) return;
  loading.value = true;
  try {
    series.value =
      props.kind === "app"
        ? await getAppMetrics(props.id, range.value)
        : await getEndpointMetrics(props.id, range.value);
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load metrics"));
  } finally {
    loading.value = false;
  }
}

watch([() => props.id, range], () => void load());
onMounted(() => void load());
</script>

<template>
  <div class="traffic">
    <div class="traffic__toolbar">
      <NSelect v-model:value="range" size="small" :options="RANGE_OPTIONS" class="traffic__range" />
      <NButton quaternary circle size="small" title="Refresh" :loading="loading" @click="load">
        <template #icon><NIcon :component="RefreshOutline" /></template>
      </NButton>
    </div>

    <NSpin :show="loading">
      <div class="traffic__grid">
        <div class="traffic__cell">
          <div class="traffic__head">
            <span class="traffic__label">RPS</span>
            <span v-if="lastPoint" class="traffic__value">{{ formatRps(lastPoint.rps) }}</span>
          </div>
          <SparklineChart
            :points="rpsPoints"
            color="var(--c-primary)"
            :format-value="formatRps"
            :format-ts="formatTs"
          />
        </div>
        <div class="traffic__cell">
          <div class="traffic__head">
            <span class="traffic__label">Avg duration</span>
            <span v-if="lastPoint" class="traffic__value">
              {{ formatDurationSeconds(lastPoint.duration_avg_seconds) }}
            </span>
          </div>
          <SparklineChart
            :points="durationPoints"
            color="var(--c-teal)"
            :format-value="formatDurationSeconds"
            :format-ts="formatTs"
          />
        </div>
        <div class="traffic__cell">
          <div class="traffic__head">
            <span class="traffic__label">Errors</span>
            <span v-if="lastPoint" class="traffic__value">
              {{ formatErrorRate(lastPoint.error_rate) }}
            </span>
          </div>
          <SparklineChart
            :points="errorPoints"
            color="var(--c-error)"
            :format-value="formatErrorRate"
            :format-ts="formatTs"
          />
        </div>
      </div>
    </NSpin>
  </div>
</template>

<style scoped>
.traffic {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.traffic__toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.traffic__range {
  width: 90px;
}

.traffic__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
}

.traffic__cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.traffic__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.traffic__label {
  font-size: 11.5px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--c-text-3);
}

.traffic__value {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
}

@media (max-width: 640px) {
  .traffic__grid {
    grid-template-columns: 1fr;
  }
}
</style>
