<script setup lang="ts">
import { computed, h, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { RefreshOutline } from "@vicons/ionicons5";
import {
  NCard,
  NDataTable,
  NDescriptions,
  NDescriptionsItem,
  NEmpty,
  NGi,
  NGrid,
  NProgress,
  NSkeleton,
  NSpace,
  NStatistic,
  NTag,
  type DataTableColumns
} from "naive-ui";
import { getSnapshotVersion, getStats, listGateways } from "../lib/api";
import { formatUnixAge, formatUnixDateTime } from "../lib/datetime";
import { notifyError } from "../lib/notify";
import type { GatewayStateItem, StatsResponse } from "../types/api";

const router = useRouter();
const loading = ref(false);
const stats = ref<StatsResponse | null>(null);
const gateways = ref<GatewayStateItem[]>([]);
const currentSnapshotVersion = ref("");
const errorMessage = ref("");

function safeNum(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

const endpointActivePercent = computed(() => {
  const total = safeNum(stats.value?.endpoints_total);
  const active = safeNum(stats.value?.endpoints_active);
  if (total <= 0) {
    return 0;
  }
  return Math.round((active / total) * 100);
});

const userAdminPercent = computed(() => {
  const total = safeNum(stats.value?.users_total);
  const admins = safeNum(stats.value?.users_admin);
  if (total <= 0) {
    return 0;
  }
  return Math.round((admins / total) * 100);
});

const topMethods = computed(() => (stats.value?.methods || []).slice(0, 6));
const topMethodTotal = computed(() => Math.max(...topMethods.value.map((x) => x.total), 1));
const kpiCards = computed(() => {
  if (!stats.value) {
    return [];
  }
  return [
    {
      title: "Applications",
      value: stats.value.apps_total,
      caption: `active ${stats.value.apps_active} / inactive ${stats.value.apps_inactive}`,
      progress: percent(stats.value.apps_active, stats.value.apps_total)
    },
    {
      title: "Endpoints",
      value: stats.value.endpoints_total,
      caption: `${endpointActivePercent.value}% active`,
      progress: endpointActivePercent.value
    },
    {
      title: "Users",
      value: stats.value.users_total,
      caption: `active ${stats.value.users_active}, admins ${stats.value.users_admin}`,
      progress: percent(stats.value.users_active, stats.value.users_total)
    },
    {
      title: "Security Surface",
      value: stats.value.root_jwt_providers,
      caption: "JWK providers",
      progress: null
    }
  ];
});

const gatewayColumns = computed<DataTableColumns<GatewayStateItem>>(() => [
  {
    title: "Gateway",
    key: "gateway_id",
    minWidth: 220,
    render(gateway) {
      return h("div", { class: "gateway-cell", title: gateway.host_name || "" }, [
        h("div", { class: "gateway-primary" }, gateway.gateway_id),
        h("div", { class: "gateway-secondary" }, gateway.host_name || "n/a")
      ]);
    }
  },
  {
    title: "Status",
    key: "status",
    width: 110,
    render(gateway) {
      return h(NTag, { size: "small", type: gateway.status === "online" ? "success" : "warning" }, { default: () => gateway.status });
    }
  },
  {
    title: "Current Applied",
    key: "snapshot_version",
    width: 150,
    render(gateway) {
      const applied = isCurrentVersionApplied(gateway);
      return h(
        NTag,
        {
          size: "small",
          type: applied ? "success" : "warning",
          title: `current: ${shortVersion(currentSnapshotVersion.value)} / gateway: ${shortVersion(gateway.snapshot_version)}`
        },
        { default: () => (applied ? "yes" : "no") }
      );
    }
  },
  {
    title: "Apply Age",
    key: "last_apply_at_unix",
    width: 130,
    render(gateway) {
      return h("span", { title: formatUnixTime(gateway.last_apply_at_unix) }, formatApplyAge(gateway.last_apply_at_unix));
    }
  },
  {
    title: "Resources",
    key: "resources",
    width: 140,
    render(gateway) {
      return h("div", { class: "gateway-resources-cell" }, [
        h("div", { class: "gateway-resources-main" }, formatMemoryBytes(gateway.memory_alloc_bytes)),
        h("div", { class: "gateway-resources-sub" }, `${gateway.goroutines_count} go`)
      ]);
    }
  }
]);

function percent(part: unknown, total: unknown): number {
  const totalNum = safeNum(total);
  if (totalNum <= 0) {
    return 0;
  }
  return Math.round((safeNum(part) / totalNum) * 100);
}

async function loadStats() {
  loading.value = true;
  errorMessage.value = "";
  try {
    const [statsRep, snapshotVersionRep] = await Promise.all([
      getStats(),
      getSnapshotVersion()
    ]);
    stats.value = statsRep;
    currentSnapshotVersion.value = snapshotVersionRep.version || "";

    try {
      const gatewaysRep = await listGateways();
      gateways.value = gatewaysRep.results || [];
    } catch {
      gateways.value = [];
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load dashboard stats";
    notifyError(errorMessage.value);
  } finally {
    loading.value = false;
  }
}

function formatUnixTime(value: unknown): string {
  return formatUnixDateTime(value, "n/a");
}

function formatApplyAge(value: unknown): string {
  return formatUnixAge(value, "n/a");
}

function formatMemoryBytes(value: unknown): string {
  const amount = Number(value);
  if (!Number.isFinite(amount) || amount <= 0) {
    return "n/a";
  }
  const mib = amount / (1024 * 1024);
  return `${mib.toFixed(mib >= 100 ? 0 : 1)} MiB`;
}

function isCurrentVersionApplied(gateway: GatewayStateItem): boolean {
  const currentVersion = (currentSnapshotVersion.value || "").trim();
  const gatewayVersion = (gateway.snapshot_version || "").trim();
  if (!currentVersion || !gatewayVersion) {
    return false;
  }
  return currentVersion === gatewayVersion;
}

function shortVersion(value: string): string {
  if (!value) {
    return "n/a";
  }
  if (value.length <= 12) {
    return value;
  }
  return `${value.slice(0, 12)}...`;
}

function openGatewayDetails(gatewayId: string): void {
  void router.push({ name: "gateway-details", params: { id: gatewayId } });
}

function gatewayRowKey(gateway: GatewayStateItem): string {
  return gateway.gateway_id;
}

function gatewayRowProps(gateway: GatewayStateItem) {
  return {
    class: "gateway-row-link",
    tabindex: 0,
    onClick: () => openGatewayDetails(gateway.gateway_id),
    onKeydown: (event: KeyboardEvent) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        openGatewayDetails(gateway.gateway_id);
      }
    }
  };
}

function methodProgress(total: number): number {
  return Math.round((total / topMethodTotal.value) * 100);
}

onMounted(() => {
  void loadStats();
});
</script>

<template>
  <div class="actions page-top-actions">
    <n-button
      secondary
      :loading="loading"
      title="Refresh Dashboard"
      aria-label="Refresh Dashboard"
      @click="loadStats"
    >
      <n-icon :component="RefreshOutline" />
    </n-button>
  </div>

  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
  <n-space vertical size="large" class="dashboard-stack">
    <n-grid v-if="loading && !stats" cols="1 s:2 l:4" responsive="screen" :x-gap="12" :y-gap="12">
      <n-gi v-for="index in 4" :key="index">
        <n-card size="small">
          <n-skeleton text style="width: 45%" />
          <n-skeleton text style="width: 30%; margin-top: 16px" />
          <n-skeleton text style="width: 70%; margin-top: 12px" />
        </n-card>
      </n-gi>
    </n-grid>

    <template v-if="stats">
      <n-grid cols="1 s:2 l:4" responsive="screen" :x-gap="12" :y-gap="12">
        <n-gi v-for="card in kpiCards" :key="card.title">
          <n-card class="dashboard-kpi-card" size="small" :bordered="true">
            <n-statistic :label="card.title" :value="card.value" />
            <div class="dashboard-kpi-meta">{{ card.caption }}</div>
            <n-progress
              v-if="card.progress !== null"
              class="dashboard-kpi-progress"
              type="line"
              :percentage="card.progress"
              :height="8"
              :show-indicator="false"
              status="success"
            />
          </n-card>
        </n-gi>
      </n-grid>

      <n-grid cols="1 l:3" responsive="screen" :x-gap="12" :y-gap="12">
        <n-gi>
          <n-card title="Service Flags" size="small">
            <n-descriptions label-placement="left" :column="1" bordered size="small">
              <n-descriptions-item label="Root Auth">
                <n-tag size="small" :type="stats.root_auth_enabled ? 'success' : 'warning'">
                  {{ stats.root_auth_enabled ? "enabled" : "disabled" }}
                </n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="Root CORS">
                <n-tag size="small" :type="stats.root_cors_enabled ? 'success' : 'warning'">
                  {{ stats.root_cors_enabled ? "enabled" : "disabled" }}
                </n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="Admin Users Ratio">{{ userAdminPercent }}%</n-descriptions-item>
            </n-descriptions>
          </n-card>
        </n-gi>

        <n-gi span="1 l:2">
          <n-card title="Endpoint Methods" size="small">
            <div v-if="topMethods.length > 0" class="dashboard-methods">
              <div v-for="method in topMethods" :key="method.method" class="method-row">
                <div class="method-head">
                  <strong>{{ method.method }}</strong>
                  <span>{{ method.active }}/{{ method.total }}</span>
                </div>
                <n-progress
                  type="line"
                  :percentage="methodProgress(method.total)"
                  :height="8"
                  :show-indicator="false"
                  status="success"
                />
              </div>
            </div>
            <n-empty v-else size="small" description="No endpoint methods yet." />
          </n-card>
        </n-gi>
      </n-grid>

      <n-card class="gateway-panel" size="small">
        <template #header>Gateways</template>
        <template #header-extra>
          <n-tag size="small" :type="currentSnapshotVersion ? 'info' : 'default'">
            current {{ shortVersion(currentSnapshotVersion) }}
          </n-tag>
        </template>
        <n-data-table
          class="gateway-table"
          size="small"
          :columns="gatewayColumns"
          :data="gateways"
          :loading="loading"
          :row-key="gatewayRowKey"
          :row-props="gatewayRowProps"
          :pagination="false"
          :bordered="false"
          :single-line="false"
        >
          <template #empty>
            <n-empty size="small" description="No gateways reported yet." />
          </template>
        </n-data-table>
      </n-card>
    </template>
  </n-space>
</template>
