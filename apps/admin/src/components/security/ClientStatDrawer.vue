<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { RouterLink } from "vue-router";
import { NAlert, NButton, NDrawer, NDrawerContent, NIcon, NSpin, NTag, useMessage } from "naive-ui";
import { BanOutline } from "@vicons/ionicons5";
import { getClientStatSeries } from "@/api/clientStat";
import { apiErrorMessage } from "@/api/http";
import { useIsMobile } from "@/composables/useIsMobile";
import { formatUnixDateTime, formatUnixRelative } from "@/lib/datetime";
import { formatCount } from "@/lib/format";
import {
  chartPoints,
  chartTsFormatter,
  fillClientStatPoints,
  formatRpm,
  formatShare,
  ipLimitLabel,
  limitShare,
  limitTone,
  rangeLabel
} from "@/lib/clientStat";
import { BLOCKED_CLIENT_SOURCE_LABEL, BLOCKED_CLIENT_SOURCE_TAG } from "@/constants/enums";
import CopyText from "@/components/common/CopyText.vue";
import DefList from "@/components/common/DefList.vue";
import DefRow from "@/components/common/DefRow.vue";
import SparklineChart from "@/components/monitoring/SparklineChart.vue";
import type { ClientStatIpLimit, ClientStatItem, ClientStatPoint } from "@/api/types";

const props = defineProps<{
  show: boolean;
  item: ClientStatItem | null;
  // the strictest Root rate limit by address, if any
  ipLimit: ClientStatIpLimit | null;
  // start of the 5 minutes recent_rpm is counted over
  recentFrom: number;
  rangeSeconds: number;
  canBlock: boolean;
}>();
const emit = defineEmits<{ "update:show": [value: boolean]; block: [item: ClientStatItem] }>();

const message = useMessage();
const isMobile = useIsMobile();

const loading = ref(false);
const points = ref<ClientStatPoint[]>([]);
const now = ref(Math.floor(Date.now() / 1000));
let requestSeq = 0;

const formatTs = computed(() => chartTsFormatter(props.rangeSeconds));
const period = computed(() => rangeLabel(props.rangeSeconds));
const block = computed(() => props.item?.block ?? null);
const isTrusted = computed(() => Boolean(props.item?.trusted));

const requestPoints = computed(() => chartPoints(points.value, "requests"));
const blockedPoints = computed(() => chartPoints(points.value, "blocked"));
const limitedPoints = computed(() => chartPoints(points.value, "limited"));
const loadedKey = ref("");

// Once the series is in, the counters come from it: they then always match the
// selected range, even when the range was changed with the drawer open.
const counters = computed(() => {
  const item = props.item;
  if (!item) return { requests: 0, blocked: 0, limited: 0 };
  if (loadedKey.value !== `${item.ip}|${props.rangeSeconds}`) {
    return { requests: item.requests, blocked: item.blocked, limited: item.limited };
  }
  return points.value.reduce(
    (sum, p) => ({
      requests: sum.requests + p.requests,
      blocked: sum.blocked + p.blocked,
      limited: sum.limited + p.limited
    }),
    { requests: 0, blocked: 0, limited: 0 }
  );
});

// Trusted clients skip rate limits. The rates come from the list row, so they
// follow the list's range.
const rates = computed(() => {
  const item = props.item;
  if (!item) return [];
  const limit = item.trusted ? null : props.ipLimit;
  return [
    { key: "peak", label: "Peak / min", value: item.peak_rpm },
    { key: "now", label: "Now / min", value: item.recent_rpm }
  ].map((rate) => {
    const share = rate.value > 0 ? limitShare(rate.value, limit) : null;
    return { ...rate, share, tone: limitTone(share) };
  });
});

const recentSpan = computed(() => {
  if (!props.recentFrom) return "the last full 5 minutes";
  const time = (unix: number) =>
    new Date(unix * 1000).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  return `${time(props.recentFrom)}–${time(props.recentFrom + 300)}`;
});

const showBlockedChart = computed(() => Boolean(counters.value.blocked || block.value));
const showLimitedChart = computed(() => Boolean(counters.value.limited));

async function load(): Promise<void> {
  const ip = props.item?.ip;
  if (!props.show || !ip) return;
  const seq = ++requestSeq;
  const range = props.rangeSeconds;
  loading.value = true;
  try {
    const rep = await getClientStatSeries(ip, range);
    if (seq !== requestSeq) return;
    now.value = Math.floor(Date.now() / 1000);
    points.value = fillClientStatPoints(rep.points, rep.step_seconds, range, now.value);
    loadedKey.value = `${ip}|${range}`;
  } catch (error) {
    if (seq === requestSeq) message.error(apiErrorMessage(error, "Failed to load the client's traffic"));
  } finally {
    if (seq === requestSeq) loading.value = false;
  }
}

watch(
  [() => props.show, () => props.item?.ip, () => props.rangeSeconds],
  ([show, ip]) => {
    if (!show) return;
    // another client: drop the previous one's chart instead of showing it while loading
    if (!loadedKey.value.startsWith(`${ip}|`)) {
      points.value = [];
      loadedKey.value = "";
    }
    void load();
  }
);

function relative(unix: number): string {
  return formatUnixRelative(unix, "—", now.value);
}
</script>

<template>
  <NDrawer
    :show="show"
    :width="isMobile ? '100%' : 520"
    placement="right"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NDrawerContent title="Client" closable>
      <div v-if="item" class="cs">
        <div class="cs__head">
          <CopyText :value="item.ip" label="Address" class="cs__ip" wrap />
          <div class="cs__tags">
            <NTag v-if="item.trusted" size="small" :bordered="false" type="success">
              trusted · {{ item.trusted }}
            </NTag>
            <NTag v-if="block" size="small" :bordered="false" type="error">blocked</NTag>
          </div>
        </div>

        <div class="cs__counters">
          <div class="cs__counter">
            <span class="cs__counter-label">Requests</span>
            <span class="cs__counter-value">{{ formatCount(counters.requests) }}</span>
          </div>
          <div class="cs__counter">
            <span class="cs__counter-label">Blocked</span>
            <span class="cs__counter-value" :class="{ 'cs__counter-value--error': counters.blocked }">
              {{ formatCount(counters.blocked) }}
            </span>
          </div>
          <div class="cs__counter">
            <span class="cs__counter-label">Rate limited</span>
            <span class="cs__counter-value" :class="{ 'cs__counter-value--warning': counters.limited }">
              {{ formatCount(counters.limited) }}
            </span>
          </div>
        </div>
        <p class="cs__note muted">
          Counted over the {{ period }}.
          <template v-if="item.first_seen_at_unix">
            First seen
            <span :title="formatUnixDateTime(item.first_seen_at_unix)">
              {{ relative(item.first_seen_at_unix) }}</span
            >,
          </template>
          last seen
          <span :title="formatUnixDateTime(item.last_seen_at_unix, '')">
            {{ relative(item.last_seen_at_unix) }}</span
          >.
          <template v-if="item.last_rule">
            Last rate limit: <span class="mono">{{ item.last_rule }}</span>.
          </template>
        </p>

        <div class="cs__counters cs__counters--rates">
          <div v-for="rate in rates" :key="rate.key" class="cs__counter">
            <span class="cs__counter-label">{{ rate.label }}</span>
            <span
              class="cs__counter-value"
              :class="rate.tone ? `cs__counter-value--${rate.tone}` : ''"
            >
              {{ formatRpm(rate.value) }}
            </span>
            <span
              v-if="rate.share !== null"
              class="cs__counter-sub"
              :class="rate.tone ? `cs__counter-value--${rate.tone}` : 'muted'"
            >
              {{ formatShare(rate.share) }} of limit
            </span>
          </div>
        </div>
        <p class="cs__note muted">
          Requests per minute: peak — the busiest 5 minutes of the {{ period }}, now —
          {{ recentSpan }}.
          <template v-if="isTrusted">A trusted client skips rate limits.</template>
          <template v-else-if="ipLimit">
            Limit: {{ ipLimitLabel(ipLimit) }}. Every gateway pod counts on its own, so with
            several pods the real ceiling can be higher.
          </template>
          <template v-else>Root has no rate limit by address.</template>
        </p>

        <NSpin :show="loading">
          <div class="cs__charts">
            <div class="cs__chart">
              <span class="cs__chart-label">Requests</span>
              <SparklineChart
                :points="requestPoints"
                :height="64"
                :format-value="formatCount"
                :format-ts="formatTs"
              />
            </div>
            <div v-if="showBlockedChart" class="cs__chart">
              <span class="cs__chart-label">Blocked</span>
              <SparklineChart
                :points="blockedPoints"
                color="var(--c-error)"
                :format-value="formatCount"
                :format-ts="formatTs"
              />
            </div>
            <div v-if="showLimitedChart" class="cs__chart">
              <span class="cs__chart-label">Rate limited</span>
              <SparklineChart
                :points="limitedPoints"
                color="var(--c-warning)"
                :format-value="formatCount"
                :format-ts="formatTs"
              />
            </div>
          </div>
        </NSpin>

        <div v-if="block" class="cs__block">
          <span class="section-label">Active block</span>
          <DefList>
            <DefRow label="Reason">{{ block.reason || "—" }}</DefRow>
            <DefRow label="Blocked by">
              <div class="cs__who">
                <NTag size="small" :bordered="false" :type="BLOCKED_CLIENT_SOURCE_TAG[block.source]">
                  {{ BLOCKED_CLIENT_SOURCE_LABEL[block.source] }}
                </NTag>
                <span>{{ block.created_by || "—" }}</span>
              </div>
            </DefRow>
            <DefRow label="Network"><span class="mono">{{ block.ip || "—" }}</span></DefRow>
            <DefRow label="Until">
              <template v-if="block.expires_at_unix">
                {{ formatUnixDateTime(block.expires_at_unix) }}
                <span class="muted">({{ relative(block.expires_at_unix) }})</span>
              </template>
              <template v-else>no end</template>
            </DefRow>
          </DefList>
          <RouterLink :to="{ name: 'blocked-clients' }" class="cs__link">
            Manage it on the Blocked clients page →
          </RouterLink>
        </div>

        <NAlert v-else-if="isTrusted" type="success" :bordered="false">
          A trusted client from the Root configuration: it skips the block list and rate limits,
          and can't be blocked.
        </NAlert>

        <div v-else-if="canBlock" class="cs__actions">
          <NButton type="error" @click="emit('block', item)">
            <template #icon><NIcon :component="BanOutline" /></template>
            Block address
          </NButton>
          <span class="muted cs__hint">The gateways start refusing it with 403 within seconds.</span>
        </div>
        <p v-else class="muted cs__hint">Administrator rights are required to block an address.</p>
      </div>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.cs {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.cs__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px 12px;
}

.cs__ip {
  font-size: 15px;
  font-weight: 600;
}

.cs__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.cs__counters {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.cs__counter {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 12px;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  background: var(--c-surface-2);
  min-width: 0;
}

.cs__counters--rates {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.cs__counter-sub {
  font-size: 11.5px;
}

.cs__counter-label {
  font-size: 11.5px;
  color: var(--c-text-3);
}

.cs__counter-value {
  font-size: 18px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.cs__counter-value--error {
  color: var(--c-error);
}

.cs__counter-value--warning {
  color: var(--c-warning);
}

.cs__note {
  margin: -6px 0 0;
  font-size: 12.5px;
  line-height: 1.5;
}

.cs__charts {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.cs__chart {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.cs__chart-label {
  font-size: 11.5px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--c-text-3);
}

.cs__block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.cs__who {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.cs__link {
  font-size: 13px;
  color: var(--c-primary);
  text-decoration: none;
}

.cs__link:hover {
  text-decoration: underline;
}

.cs__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 12px;
}

.cs__hint {
  margin: 0;
  font-size: 12.5px;
}
</style>
