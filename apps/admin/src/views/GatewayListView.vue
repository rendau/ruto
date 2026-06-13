<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from "vue";
import { NButton, NDataTable, NIcon, NTag, useMessage, type DataTableColumns } from "naive-ui";
import { RefreshOutline } from "@vicons/ionicons5";
import { listGateways } from "@/api/gateway";
import { apiErrorMessage } from "@/api/http";
import { useSnapshotStore } from "@/stores/snapshot";
import { formatUnixAge } from "@/lib/datetime";
import { formatBytes } from "@/lib/format";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import GatewayDetailDrawer from "@/components/gateway/GatewayDetailDrawer.vue";
import type { GatewayStateItem } from "@/api/types";

const message = useMessage();
const snapshotStore = useSnapshotStore();

const gateways = ref<GatewayStateItem[]>([]);
const loading = ref(false);
const showDetail = ref(false);
const selected = ref<GatewayStateItem | null>(null);
let timer: ReturnType<typeof setInterval> | undefined;

const counts = computed(() => {
  const result = { total: gateways.value.length, online: 0, stale: 0, offline: 0 };
  for (const gw of gateways.value) {
    result[gw.status] += 1;
  }
  return result;
});

const statCards = computed(() => [
  { key: "total", label: "Total", value: counts.value.total, tone: "default" },
  { key: "online", label: "Online", value: counts.value.online, tone: "online" },
  { key: "stale", label: "Stale", value: counts.value.stale, tone: "stale" },
  { key: "offline", label: "Offline", value: counts.value.offline, tone: "offline" }
]);

const columns: DataTableColumns<GatewayStateItem> = [
  {
    title: "Gateway",
    key: "gateway_id",
    render: (row) =>
      h("div", { class: "gw-cell" }, [
        h("span", { class: "gw-cell__id mono" }, row.gateway_id || "—"),
        h("span", { class: "gw-cell__host muted" }, row.host_name || "")
      ])
  },
  {
    title: "Status",
    key: "status",
    width: 110,
    render: (row) =>
      h(
        NTag,
        {
          size: "small",
          bordered: false,
          type:
            row.status === "online" ? "success" : row.status === "stale" ? "warning" : "error"
        },
        { default: () => row.status }
      )
  },
  {
    title: "Applied",
    key: "snapshot_version",
    width: 110,
    render: (row) =>
      h(
        NTag,
        {
          size: "small",
          bordered: false,
          type: row.snapshot_version === snapshotStore.version ? "success" : "warning"
        },
        { default: () => (row.snapshot_version === snapshotStore.version ? "current" : "behind") }
      )
  },
  {
    title: "Applied age",
    key: "last_apply_at_unix",
    width: 120,
    render: (row) => formatUnixAge(row.last_apply_at_unix)
  },
  {
    title: "Last seen",
    key: "last_seen_at_unix",
    width: 120,
    render: (row) => formatUnixAge(row.last_seen_at_unix)
  },
  {
    title: "Resources",
    key: "resources",
    width: 150,
    render: (row) =>
      h("span", { class: "muted" }, `${formatBytes(row.memory_alloc_bytes)} · ${row.goroutines_count} gr`)
  }
];

function rowProps(row: GatewayStateItem) {
  return {
    class: "clickable-row",
    onClick: () => {
      selected.value = row;
      showDetail.value = true;
    }
  };
}

async function load(): Promise<void> {
  loading.value = true;
  try {
    const [rep] = await Promise.all([listGateways(), snapshotStore.loadVersion()]);
    gateways.value = rep.results ?? [];
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load gateways"));
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void load();
  timer = setInterval(() => void load(), 10000);
});

onBeforeUnmount(() => {
  if (timer) clearInterval(timer);
});
</script>

<template>
  <PageContainer :width="1080">
    <div class="page-head">
      <div>
        <h1 class="page-head__title">Gateways</h1>
        <p class="page-head__sub muted">
          Connected data-plane instances · target
          <span class="mono">{{ snapshotStore.version.slice(0, 10) || "—" }}</span>
        </p>
      </div>
      <NButton :loading="loading" @click="load">
        <template #icon><NIcon :component="RefreshOutline" /></template>
        Refresh
      </NButton>
    </div>

    <div class="stat-grid">
      <div v-for="card in statCards" :key="card.key" class="stat" :class="`stat--${card.tone}`">
        <span class="stat__value">{{ card.value }}</span>
        <span class="stat__label">{{ card.label }}</span>
      </div>
    </div>

    <SectionCard>
      <NDataTable
        :columns="columns"
        :data="gateways"
        :loading="loading"
        :row-props="rowProps"
        :bordered="false"
        :scroll-x="820"
      />
      <p v-if="!loading && gateways.length === 0" class="muted gw-empty">
        No gateways are currently connected.
      </p>
    </SectionCard>

    <GatewayDetailDrawer v-model:show="showDetail" :gateway="selected" />
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
  font-size: 13px;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

.stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px;
  border: 1px solid var(--c-border);
  border-radius: 12px;
  background: var(--c-surface);
}

.stat__value {
  font-size: 26px;
  font-weight: 700;
}

.stat__label {
  font-size: 12.5px;
  color: var(--c-text-3);
}

.stat--online .stat__value {
  color: var(--c-success);
}

.stat--stale .stat__value {
  color: var(--c-warning);
}

.stat--offline .stat__value {
  color: var(--c-error);
}

.gw-empty {
  padding: 8px 0;
}

:deep(.clickable-row) {
  cursor: pointer;
}

:deep(.gw-cell) {
  display: flex;
  flex-direction: column;
}

:deep(.gw-cell__id) {
  font-size: 13px;
}

:deep(.gw-cell__host) {
  font-size: 11.5px;
}

@media (max-width: 700px) {
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
