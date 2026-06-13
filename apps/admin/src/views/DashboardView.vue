<script setup lang="ts">
import { computed, h, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import {
  NButton,
  NDataTable,
  NIcon,
  NSpin,
  NTag,
  useMessage,
  type DataTableColumns
} from "naive-ui";
import {
  AppsOutline,
  GitNetworkOutline,
  PeopleOutline,
  RefreshOutline,
  ShieldCheckmarkOutline
} from "@vicons/ionicons5";
import { getStats } from "@/api/stats";
import { listGateways } from "@/api/gateway";
import { apiErrorMessage } from "@/api/http";
import { useSnapshotStore } from "@/stores/snapshot";
import { formatDuration, formatUnixAge } from "@/lib/datetime";
import { formatBytes } from "@/lib/format";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import type { GatewayStateItem, StatsResponse } from "@/api/types";

const router = useRouter();
const message = useMessage();
const snapshotStore = useSnapshotStore();

const stats = ref<StatsResponse | null>(null);
const gateways = ref<GatewayStateItem[]>([]);
const loading = ref(false);

const kpis = computed(() => {
  const s = stats.value;
  return [
    {
      key: "apps",
      label: "Applications",
      icon: AppsOutline,
      value: s?.apps_total ?? 0,
      sub: `${s?.apps_active ?? 0} active · ${s?.apps_inactive ?? 0} inactive`,
      ratio: ratio(s?.apps_active, s?.apps_total)
    },
    {
      key: "endpoints",
      label: "Endpoints",
      icon: GitNetworkOutline,
      value: s?.endpoints_total ?? 0,
      sub: `${s?.endpoints_active ?? 0} active · ${s?.endpoints_inactive ?? 0} inactive`,
      ratio: ratio(s?.endpoints_active, s?.endpoints_total)
    },
    {
      key: "users",
      label: "Users",
      icon: PeopleOutline,
      value: s?.users_total ?? 0,
      sub: `${s?.users_active ?? 0} active · ${s?.users_admin ?? 0} admin`,
      ratio: ratio(s?.users_active, s?.users_total)
    },
    {
      key: "security",
      label: "Security surface",
      icon: ShieldCheckmarkOutline,
      value: s?.root_jwt_providers ?? 0,
      sub: "JWT providers configured",
      ratio: s?.root_auth_enabled ? 1 : 0
    }
  ];
});

const serviceFlags = computed(() => {
  const s = stats.value;
  return [
    { label: "Root auth", on: Boolean(s?.root_auth_enabled), onText: "Enabled", offText: "Disabled" },
    { label: "Root CORS", on: Boolean(s?.root_cors_enabled), onText: "Enabled", offText: "Disabled" }
  ];
});

const topMethods = computed(() => {
  const methods = [...(stats.value?.methods ?? [])].sort((a, b) => b.total - a.total).slice(0, 8);
  const max = Math.max(1, ...methods.map((m) => m.total));
  return methods.map((m) => ({ ...m, pct: Math.round((m.total / max) * 100) }));
});

const uptime = computed(() => formatDuration(stats.value?.core_uptime_seconds ?? 0));

function ratio(active?: number, total?: number): number {
  if (!total || total <= 0) return 0;
  return Math.min(1, Math.max(0, (active ?? 0) / total));
}

const gatewayColumns: DataTableColumns<GatewayStateItem> = [
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
    width: 120,
    render: (row) =>
      h(
        NTag,
        {
          size: "small",
          bordered: false,
          type: row.snapshot_version === snapshotStore.version ? "success" : "default"
        },
        { default: () => (row.snapshot_version === snapshotStore.version ? "current" : "stale") }
      )
  },
  {
    title: "Applied age",
    key: "last_apply_at_unix",
    width: 120,
    render: (row) => formatUnixAge(row.last_apply_at_unix)
  },
  {
    title: "Resources",
    key: "resources",
    width: 150,
    render: (row) =>
      h("span", { class: "muted" }, `${formatBytes(row.memory_alloc_bytes)} · ${row.goroutines_count} gr`)
  }
];

async function load(): Promise<void> {
  loading.value = true;
  try {
    const [statsRep, gatewaysRep] = await Promise.all([
      getStats(),
      listGateways().catch(() => ({ results: [] }))
    ]);
    stats.value = statsRep;
    gateways.value = gatewaysRep.results ?? [];
    void snapshotStore.loadVersion();
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load dashboard"));
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <PageContainer>
    <div class="page-head">
      <div>
        <h1 class="page-head__title">Dashboard</h1>
        <p class="page-head__sub muted">Overview of the gateway control plane</p>
      </div>
      <NButton :loading="loading" @click="load">
        <template #icon><NIcon :component="RefreshOutline" /></template>
        Refresh
      </NButton>
    </div>

    <NSpin :show="loading">
      <div class="kpi-grid">
        <div v-for="kpi in kpis" :key="kpi.key" class="kpi">
          <div class="kpi__top">
            <span class="kpi__label">{{ kpi.label }}</span>
            <NIcon :component="kpi.icon" :size="18" class="kpi__icon" />
          </div>
          <div class="kpi__value">{{ kpi.value }}</div>
          <div class="kpi__sub muted">{{ kpi.sub }}</div>
          <div class="kpi__bar">
            <div class="kpi__bar-fill" :style="{ width: `${Math.round(kpi.ratio * 100)}%` }" />
          </div>
        </div>
      </div>

      <div class="dash-grid">
        <SectionCard title="Request methods" description="Endpoints grouped by HTTP method">
          <div v-if="topMethods.length" class="methods">
            <div v-for="m in topMethods" :key="m.method" class="method-row">
              <span class="method-row__name mono">{{ m.method || "—" }}</span>
              <div class="method-row__track">
                <div class="method-row__fill" :style="{ width: `${m.pct}%` }" />
              </div>
              <span class="method-row__count">
                <strong>{{ m.active }}</strong
                ><span class="muted">/{{ m.total }}</span>
              </span>
            </div>
          </div>
          <p v-else class="muted">No endpoints registered yet.</p>
        </SectionCard>

        <SectionCard title="Service state">
          <div class="flags">
            <div v-for="flag in serviceFlags" :key="flag.label" class="flag">
              <span class="flag__label">{{ flag.label }}</span>
              <NTag size="small" :bordered="false" :type="flag.on ? 'success' : 'default'">
                {{ flag.on ? flag.onText : flag.offText }}
              </NTag>
            </div>
            <div class="flag">
              <span class="flag__label">Core uptime</span>
              <span class="flag__value mono">{{ uptime }}</span>
            </div>
            <div class="flag">
              <span class="flag__label">Snapshot</span>
              <span class="flag__value mono">{{ snapshotStore.version.slice(0, 12) || "—" }}</span>
            </div>
          </div>
        </SectionCard>
      </div>

      <SectionCard title="Gateways" :description="`${gateways.length} connected`">
        <template #extra>
          <NButton size="small" tertiary @click="router.push({ name: 'gateways' })">
            Manage gateways
          </NButton>
        </template>
        <NDataTable
          v-if="gateways.length"
          :columns="gatewayColumns"
          :data="gateways"
          :bordered="false"
          size="small"
          :scroll-x="720"
        />
        <p v-else class="muted">No gateways are currently connected.</p>
      </SectionCard>
    </NSpin>
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

.kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

.kpi {
  padding: 16px;
  border: 1px solid var(--c-border);
  border-radius: 12px;
  background: var(--c-surface);
}

.kpi__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.kpi__label {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--c-text-2);
}

.kpi__icon {
  color: var(--c-text-3);
}

.kpi__value {
  margin-top: 8px;
  font-size: 30px;
  font-weight: 700;
  line-height: 1.1;
}

.kpi__sub {
  margin-top: 4px;
  font-size: 12px;
}

.kpi__bar {
  margin-top: 12px;
  height: 5px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.06);
  overflow: hidden;
}

.kpi__bar-fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, #63e2b7, #4bb592);
  transition: width 0.4s ease;
}

.dash-grid {
  display: grid;
  grid-template-columns: 1.6fr 1fr;
  gap: 14px;
}

.methods {
  display: flex;
  flex-direction: column;
  gap: 11px;
}

.method-row {
  display: grid;
  grid-template-columns: 64px 1fr 64px;
  gap: 12px;
  align-items: center;
}

.method-row__name {
  font-size: 12.5px;
  font-weight: 700;
  color: var(--c-text-2);
}

.method-row__track {
  height: 8px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.05);
  overflow: hidden;
}

.method-row__fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, #63e2b7, #4bb592);
}

.method-row__count {
  text-align: right;
  font-size: 13px;
}

.flags {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.flag {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}

.flag__label {
  font-size: 13px;
  color: var(--c-text-2);
}

.flag__value {
  font-size: 13px;
  color: var(--c-text);
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

@media (max-width: 1000px) {
  .kpi-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .dash-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .kpi-grid {
    grid-template-columns: 1fr;
  }
}
</style>
