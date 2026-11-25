<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch, type VNodeChild } from "vue";
import {
  NButton,
  NDataTable,
  NIcon,
  NRadioButton,
  NRadioGroup,
  NSpin,
  NTab,
  NTabs,
  NTag,
  useMessage,
  type DataTableColumns
} from "naive-ui";
import { RefreshOutline } from "@vicons/ionicons5";
import { getClientStatOverview, getClientStatTop } from "@/api/clientStat";
import { apiErrorMessage } from "@/api/http";
import { useAuthStore } from "@/stores/auth";
import { useIsMobile } from "@/composables/useIsMobile";
import { formatDuration, formatUnixDateTime, formatUnixRelative } from "@/lib/datetime";
import { formatCompact, formatCount } from "@/lib/format";
import {
  CLIENT_STAT_DEFAULT_RANGE,
  CLIENT_STAT_RANGE_OPTIONS,
  chartPoints,
  chartTsFormatter,
  fillClientStatPoints,
  rangeLabel
} from "@/lib/clientStat";
import { BLOCKED_CLIENT_SOURCE_LABEL, BLOCKED_CLIENT_SOURCE_TAG } from "@/constants/enums";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import SparklineChart from "@/components/monitoring/SparklineChart.vue";
import BlockClientModal from "@/components/blockedclient/BlockClientModal.vue";
import ClientStatDrawer from "@/components/security/ClientStatDrawer.vue";
import type {
  BlockedClientMain,
  ClientStatItem,
  ClientStatKind,
  ClientStatOverviewRep
} from "@/api/types";

const TOP_LIMIT = 50;

const message = useMessage();
const authStore = useAuthStore();
const isMobile = useIsMobile();
const canBlock = computed(() => authStore.isAdmin);

const range = ref(CLIENT_STAT_DEFAULT_RANGE);
// relative times ("5m ago") are counted from this moment, moved on every load
const now = ref(Math.floor(Date.now() / 1000));

// ---- Overview: tiles and charts ---------------------------------------------

const overview = ref<ClientStatOverviewRep | null>(null);
const overviewLoading = ref(false);
let overviewSeq = 0;

async function loadOverview(): Promise<void> {
  const seq = ++overviewSeq;
  const rangeSeconds = range.value;
  overviewLoading.value = true;
  try {
    const rep = await getClientStatOverview(rangeSeconds);
    if (seq !== overviewSeq) return;
    overview.value = rep;
    now.value = Math.floor(Date.now() / 1000);
  } catch (error) {
    if (seq === overviewSeq) message.error(apiErrorMessage(error, "Failed to load client statistics"));
  } finally {
    if (seq === overviewSeq) overviewLoading.value = false;
  }
}

const totals = computed(() => overview.value?.totals);
const period = computed(() => rangeLabel(range.value));
const formatTs = computed(() => chartTsFormatter(range.value));
const stepText = computed(() => formatDuration(overview.value?.step_seconds || 300));

const tiles = computed(() => [
  {
    key: "requests",
    label: "Requests",
    value: totals.value?.requests ?? 0,
    sub: "all requests the gateways got",
    tone: ""
  },
  {
    key: "blocked",
    label: "Blocked",
    value: totals.value?.blocked ?? 0,
    sub: "refused by the block list",
    tone: "error"
  },
  {
    key: "limited",
    label: "Rate limited",
    value: totals.value?.limited ?? 0,
    sub: "refused by a rate limit",
    tone: "warning"
  },
  {
    key: "trusted",
    label: "Trusted requests",
    value: totals.value?.trusted ?? 0,
    sub: "from trusted clients",
    tone: ""
  },
  {
    key: "clients",
    label: "Distinct clients",
    value: totals.value?.clients ?? 0,
    sub: "addresses that called",
    tone: ""
  }
]);

const timeline = computed(() => {
  const rep = overview.value;
  if (!rep) return [];
  return fillClientStatPoints(rep.points, rep.step_seconds, range.value, now.value);
});
const requestPoints = computed(() => chartPoints(timeline.value, "requests"));
const blockedPoints = computed(() => chartPoints(timeline.value, "blocked"));
const limitedPoints = computed(() => chartPoints(timeline.value, "limited"));

// A trusted client's own series may carry its count in requests or in trusted —
// take whichever is filled so the chart doesn't depend on that detail.
const trustedCharts = computed(() => {
  const rep = overview.value;
  if (!rep) return [];
  return rep.trusted.map((item) => ({
    name: item.name,
    requests: item.requests,
    points: fillClientStatPoints(item.points, rep.step_seconds, range.value, now.value).map((p) => ({
      ts: p.ts,
      value: Math.max(p.requests, p.trusted)
    }))
  }));
});

// ---- Top lists, one per tab -------------------------------------------------

interface TopState {
  rows: ClientStatItem[];
  loading: boolean;
  // the range the rows were loaded for; 0 = never loaded or outdated
  loadedRange: number;
  seq: number;
}

const KINDS: ClientStatKind[] = ["requests", "blocked", "limited", "trusted"];

const TAB_LABEL: Record<ClientStatKind, string> = {
  requests: "Busiest clients",
  blocked: "Blocked",
  limited: "Rate limited",
  trusted: "Trusted"
};

const TAB_HINT: Record<ClientStatKind, string> = {
  requests: "Who sent the most requests, busiest first.",
  blocked:
    "Blocked addresses that kept sending requests — the gateways refused each one with 403.",
  limited: "Who hit a rate limit and got 429. “Last rule” is the limit that refused them last.",
  trusted:
    "Trusted clients from the Root configuration: they skip the block list and rate limits."
};

const EMPTY_TEXT: Record<ClientStatKind, string> = {
  requests: "No requests in this period.",
  blocked: "Nobody was blocked in this period.",
  limited: "No rate-limit rejections in this period.",
  trusted: "No requests from trusted clients in this period."
};

const tab = ref<ClientStatKind>("requests");
const top = reactive(
  Object.fromEntries(
    KINDS.map((kind) => [kind, { rows: [], loading: false, loadedRange: 0, seq: 0 }])
  ) as unknown as Record<ClientStatKind, TopState>
);
const current = computed(() => top[tab.value]);

async function loadTop(kind: ClientStatKind): Promise<void> {
  const state = top[kind];
  const seq = ++state.seq;
  const rangeSeconds = range.value;
  state.loading = true;
  try {
    const rep = await getClientStatTop(rangeSeconds, kind, TOP_LIMIT);
    if (seq !== state.seq) return;
    state.rows = rep.results;
    state.loadedRange = rangeSeconds;
    now.value = Math.floor(Date.now() / 1000);
    refreshSelected(rep.results);
  } catch (error) {
    if (seq === state.seq) message.error(apiErrorMessage(error, "Failed to load the client list"));
  } finally {
    if (seq === state.seq) state.loading = false;
  }
}

function load(): void {
  for (const kind of KINDS) {
    top[kind].loadedRange = 0;
  }
  void loadOverview();
  void loadTop(tab.value);
}

watch(range, load);

watch(tab, (kind) => {
  if (top[kind].loadedRange !== range.value && !top[kind].loading) void loadTop(kind);
});

onMounted(load);

// ---- Drawer and block modal -------------------------------------------------

const selected = ref<ClientStatItem | null>(null);
const showDrawer = ref(false);
const showBlock = ref(false);
const blockIp = ref("");

function openClient(row: ClientStatItem): void {
  selected.value = row;
  showDrawer.value = true;
}

// Fresh rows (another range, a new block) replace the one shown in the drawer.
function refreshSelected(rows: ClientStatItem[]): void {
  if (!selected.value) return;
  const ip = selected.value.ip;
  const fresh = rows.find((row) => row.ip === ip);
  if (fresh) selected.value = fresh;
}

function openBlock(item: ClientStatItem): void {
  blockIp.value = item.ip;
  showBlock.value = true;
}

function onBlocked(row: BlockedClientMain): void {
  if (selected.value && selected.value.ip === blockIp.value) {
    selected.value = {
      ...selected.value,
      block: {
        id: row.id,
        ip: row.ip,
        reason: row.reason,
        source: row.source,
        created_by: row.created_by,
        expires_at_unix: row.expires_at_unix
      }
    };
  }
  load();
}

// ---- Table ------------------------------------------------------------------

function relative(unix: number): string {
  return formatUnixRelative(unix, "—", now.value);
}

function untilText(item: ClientStatItem): string {
  if (!item.block) return "not blocked now";
  return item.block.expires_at_unix ? relative(item.block.expires_at_unix) : "no end";
}

function renderAddress(row: ClientStatItem, withTags: boolean): VNodeChild {
  return h("div", { class: "sec-address" }, [
    h(
      NButton,
      {
        text: true,
        type: "primary",
        class: "mono sec-address__btn",
        title: "Show this client's traffic",
        onClick: () => openClient(row)
      },
      { default: () => row.ip || "—" }
    ),
    withTags && row.trusted
      ? h(
          NTag,
          { size: "tiny", bordered: false, type: "success" },
          { default: () => `trusted · ${row.trusted}` }
        )
      : null,
    withTags && row.block
      ? h(NTag, { size: "tiny", bordered: false, type: "error" }, { default: () => "blocked" })
      : null
  ]);
}

function countColumn(
  title: string,
  key: "requests" | "blocked" | "limited",
  tone = ""
): DataTableColumns<ClientStatItem>[number] {
  return {
    title,
    key,
    width: 120,
    align: "right",
    sorter: (a, b) => a[key] - b[key],
    render: (row) =>
      h(
        "span",
        { class: ["sec-num", row[key] && tone ? `sec-num--${tone}` : ""] },
        formatCount(row[key])
      )
  };
}

const lastSeenColumn: DataTableColumns<ClientStatItem>[number] = {
  title: "Last seen",
  key: "last_seen_at_unix",
  width: 120,
  sorter: (a, b) => a.last_seen_at_unix - b.last_seen_at_unix,
  render: (row) =>
    h(
      "span",
      { class: "muted", title: formatUnixDateTime(row.last_seen_at_unix, "") },
      relative(row.last_seen_at_unix)
    )
};

const columns = computed<DataTableColumns<ClientStatItem>>(() => {
  const address = (withTags: boolean, title = "Address"): DataTableColumns<ClientStatItem>[number] => ({
    title,
    key: "ip",
    minWidth: 220,
    render: (row) => renderAddress(row, withTags)
  });
  switch (tab.value) {
    case "blocked":
      return [
        address(false),
        countColumn("Blocked", "blocked", "error"),
        {
          title: "Reason",
          key: "reason",
          minWidth: 180,
          ellipsis: { tooltip: true },
          render: (row) => row.block?.reason || "—"
        },
        {
          title: "Blocked by",
          key: "created_by",
          width: 200,
          render: (row) =>
            row.block
              ? h("div", { class: "sec-who" }, [
                  h(
                    NTag,
                    {
                      size: "small",
                      bordered: false,
                      type: BLOCKED_CLIENT_SOURCE_TAG[row.block.source],
                      title:
                        row.block.source === "agent"
                          ? "Blocked by an agent with an API key (pulse, an MCP agent)"
                          : "Blocked in the admin"
                    },
                    { default: () => BLOCKED_CLIENT_SOURCE_LABEL[row.block?.source ?? "admin"] }
                  ),
                  h(
                    "span",
                    { class: "sec-who__name", title: row.block.created_by },
                    row.block.created_by || "—"
                  )
                ])
              : "—"
        },
        {
          title: "Until",
          key: "expires_at_unix",
          width: 130,
          render: (row) =>
            h(
              "span",
              {
                class: row.block ? "" : "muted",
                title: row.block?.expires_at_unix
                  ? formatUnixDateTime(row.block.expires_at_unix)
                  : ""
              },
              untilText(row)
            )
        },
        lastSeenColumn
      ];
    case "limited":
      return [
        address(true),
        countColumn("Rate limited", "limited", "warning"),
        countColumn("Requests", "requests"),
        {
          title: "Last rule",
          key: "last_rule",
          minWidth: 160,
          ellipsis: { tooltip: true },
          render: (row) => h("span", { class: "mono" }, row.last_rule || "—")
        },
        lastSeenColumn
      ];
    case "trusted":
      return [
        {
          title: "Name",
          key: "trusted",
          minWidth: 160,
          render: (row) => h("strong", null, row.trusted || "—")
        },
        address(false, "Network"),
        countColumn("Requests", "requests"),
        lastSeenColumn
      ];
    default:
      return [
        address(true),
        countColumn("Requests", "requests"),
        countColumn("Blocked", "blocked", "error"),
        countColumn("Rate limited", "limited", "warning"),
        lastSeenColumn
      ];
  }
});

function rowKey(row: ClientStatItem): string {
  return `${row.ip}|${row.trusted}`;
}
</script>

<template>
  <PageContainer :width="1180">
    <div class="page-head">
      <div>
        <h1 class="page-head__title">Security</h1>
        <p class="page-head__sub muted">
          Who calls the gateways: blocked clients that keep knocking, rate-limit hits, trusted
          clients and the busiest addresses. Collected by the gateways every 30 s, kept in
          5-minute steps for 7 days.
        </p>
      </div>
      <div class="page-head__actions">
        <NRadioGroup v-model:value="range" size="small" aria-label="Period">
          <NRadioButton
            v-for="option in CLIENT_STAT_RANGE_OPTIONS"
            :key="option.value"
            :value="option.value"
            :label="option.label"
          />
        </NRadioGroup>
        <NButton tertiary :loading="overviewLoading || current.loading" @click="load">
          <template #icon><NIcon :component="RefreshOutline" /></template>
          Refresh
        </NButton>
      </div>
    </div>

    <div class="sec-tiles">
      <div v-for="tile in tiles" :key="tile.key" class="sec-tile">
        <span class="sec-tile__label">{{ tile.label }}</span>
        <span
          class="sec-tile__value"
          :class="{ [`sec-tile__value--${tile.tone}`]: tile.tone && tile.value }"
          :title="formatCount(tile.value)"
        >
          {{ overview ? formatCompact(tile.value) : "—" }}
        </span>
        <span class="sec-tile__sub muted">{{ tile.sub }}</span>
      </div>
    </div>

    <SectionCard title="Traffic" :description="`All clients, ${period}, per ${stepText} step`">
      <NSpin :show="overviewLoading && !overview">
        <div class="sec-charts">
          <div class="sec-chart">
            <div class="sec-chart__head">
              <span class="sec-chart__label">Requests</span>
              <span class="sec-chart__value">{{ formatCount(totals?.requests ?? 0) }}</span>
            </div>
            <SparklineChart
              :points="requestPoints"
              :height="80"
              :format-value="formatCount"
              :format-ts="formatTs"
            />
          </div>
          <div class="sec-charts__pair">
            <div class="sec-chart">
              <div class="sec-chart__head">
                <span class="sec-chart__label">Blocked</span>
                <span class="sec-chart__value">{{ formatCount(totals?.blocked ?? 0) }}</span>
              </div>
              <SparklineChart
                :points="blockedPoints"
                color="var(--c-error)"
                :format-value="formatCount"
                :format-ts="formatTs"
              />
            </div>
            <div class="sec-chart">
              <div class="sec-chart__head">
                <span class="sec-chart__label">Rate limited</span>
                <span class="sec-chart__value">{{ formatCount(totals?.limited ?? 0) }}</span>
              </div>
              <SparklineChart
                :points="limitedPoints"
                color="var(--c-warning)"
                :format-value="formatCount"
                :format-ts="formatTs"
              />
            </div>
          </div>
        </div>
      </NSpin>
    </SectionCard>

    <SectionCard
      title="Trusted clients"
      description="Requests of each trusted client from the Root configuration, busiest first"
    >
      <div v-if="trustedCharts.length" class="sec-trusted">
        <div v-for="item in trustedCharts" :key="item.name" class="sec-chart">
          <div class="sec-chart__head">
            <span class="sec-chart__name" :title="item.name">{{ item.name }}</span>
            <span class="sec-chart__value">{{ formatCount(item.requests) }}</span>
          </div>
          <SparklineChart
            :points="item.points"
            color="#70c0e8"
            :format-value="formatCount"
            :format-ts="formatTs"
          />
        </div>
      </div>
      <p v-else-if="overview" class="muted sec-empty-line">
        No requests from trusted clients in this period.
      </p>
    </SectionCard>

    <SectionCard>
      <NTabs v-model:value="tab" type="line" class="sec-tabs">
        <NTab v-for="kind in KINDS" :key="kind" :name="kind" :tab="TAB_LABEL[kind]" />
      </NTabs>
      <p class="sec-tab-hint muted">
        {{ TAB_HINT[tab] }} Top {{ TOP_LIMIT }}, {{ period }}. Click an address for details.
      </p>

      <template v-if="isMobile">
        <NSpin :show="current.loading">
          <div v-if="current.rows.length" class="sec-cards">
            <div v-for="row in current.rows" :key="rowKey(row)" class="sec-card">
              <div class="sec-card__head">
                <NButton text type="primary" class="mono sec-card__ip" @click="openClient(row)">
                  {{ row.ip || "—" }}
                </NButton>
                <NTag v-if="row.trusted" size="tiny" :bordered="false" type="success">
                  trusted · {{ row.trusted }}
                </NTag>
                <NTag v-if="row.block" size="tiny" :bordered="false" type="error">blocked</NTag>
              </div>
              <div class="sec-card__meta">
                <span>{{ formatCount(row.requests) }} requests</span>
                <span v-if="row.blocked" class="sec-num--error">
                  {{ formatCount(row.blocked) }} blocked
                </span>
                <span v-if="row.limited" class="sec-num--warning">
                  {{ formatCount(row.limited) }} rate limited
                </span>
              </div>
              <p v-if="tab === 'blocked' && row.block" class="sec-card__line">
                {{ row.block.reason || "—" }}
                <span class="muted">
                  · {{ BLOCKED_CLIENT_SOURCE_LABEL[row.block.source] }}
                  {{ row.block.created_by }} · until {{ untilText(row) }}
                </span>
              </p>
              <p v-else-if="tab === 'limited' && row.last_rule" class="sec-card__line">
                <span class="muted">last rule</span> <span class="mono">{{ row.last_rule }}</span>
              </p>
              <span class="sec-card__seen muted" :title="formatUnixDateTime(row.last_seen_at_unix, '')">
                last seen {{ relative(row.last_seen_at_unix) }}
              </span>
            </div>
          </div>
          <EmptyState
            v-else-if="current.loadedRange"
            :description="EMPTY_TEXT[tab]"
            size="medium"
          />
        </NSpin>
      </template>
      <NDataTable
        v-else
        :columns="columns"
        :data="current.rows"
        :loading="current.loading"
        :row-key="rowKey"
        :bordered="false"
        :scroll-x="860"
      >
        <template #empty>
          <EmptyState :description="EMPTY_TEXT[tab]" size="medium" />
        </template>
      </NDataTable>
    </SectionCard>

    <ClientStatDrawer
      v-model:show="showDrawer"
      :item="selected"
      :range-seconds="range"
      :can-block="canBlock"
      @block="openBlock"
    />
    <BlockClientModal v-model:show="showBlock" :ip="blockIp" @saved="onBlocked" />
  </PageContainer>
</template>

<style scoped>
.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.page-head__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
}

.page-head__sub {
  margin: 3px 0 0;
  max-width: 640px;
  font-size: 13px;
}

.page-head__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.sec-tiles {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}

.sec-tile {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 14px 16px;
  border: 1px solid var(--c-border);
  border-radius: 12px;
  background: var(--c-surface);
  min-width: 0;
}

.sec-tile__label {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--c-text-2);
}

.sec-tile__value {
  margin-top: 4px;
  font-size: 26px;
  font-weight: 700;
  line-height: 1.15;
  font-variant-numeric: tabular-nums;
}

.sec-tile__value--error {
  color: var(--c-error);
}

.sec-tile__value--warning {
  color: var(--c-warning);
}

.sec-tile__sub {
  font-size: 12px;
}

.sec-charts {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.sec-charts__pair {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.sec-trusted {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 18px;
}

.sec-chart {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.sec-chart__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}

.sec-chart__label {
  font-size: 11.5px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--c-text-3);
}

.sec-chart__name {
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sec-chart__value {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  flex-shrink: 0;
}

.sec-empty-line {
  margin: 0;
  font-size: 13px;
}

.sec-tab-hint {
  margin: 10px 0 14px;
  font-size: 12.5px;
}

:deep(.sec-address) {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 8px;
}

:deep(.sec-address__btn) {
  font-family: var(--font-mono);
}

:deep(.sec-num) {
  font-variant-numeric: tabular-nums;
}

:deep(.sec-num--error) {
  color: var(--c-error);
}

:deep(.sec-num--warning) {
  color: var(--c-warning);
}

:deep(.sec-who) {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

:deep(.sec-who__name) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sec-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 60px;
}

.sec-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--c-border);
  border-radius: 11px;
  background: var(--c-surface);
}

.sec-card__head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 8px;
}

.sec-card__ip {
  font-family: var(--font-mono);
  font-size: 14px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.sec-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 12px;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.sec-card__line {
  margin: 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}

.sec-card__seen {
  font-size: 12px;
}

@media (max-width: 1000px) {
  .sec-tiles {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .sec-tiles {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .sec-tile__value {
    font-size: 22px;
  }

  .sec-charts__pair {
    grid-template-columns: 1fr;
  }
}
</style>
