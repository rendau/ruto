<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { RefreshOutline } from "@vicons/ionicons5";
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
  <p v-if="loading && !stats" class="muted">Loading dashboard...</p>

  <template v-if="stats">
    <section class="dashboard-kpis">
      <article class="kpi-card">
        <div class="kpi-title">Applications</div>
        <div class="kpi-value">{{ stats.apps_total }}</div>
        <div class="kpi-sub">active {{ stats.apps_active }} / inactive {{ stats.apps_inactive }}</div>
      </article>
      <article class="kpi-card">
        <div class="kpi-title">Endpoints</div>
        <div class="kpi-value">{{ stats.endpoints_total }}</div>
        <div class="kpi-sub">{{ endpointActivePercent }}% active</div>
      </article>
      <article class="kpi-card">
        <div class="kpi-title">Users</div>
        <div class="kpi-value">{{ stats.users_total }}</div>
        <div class="kpi-sub">active {{ stats.users_active }}, admins {{ stats.users_admin }}</div>
      </article>
      <article class="kpi-card">
        <div class="kpi-title">Security Surface</div>
        <div class="kpi-value">{{ stats.root_jwt_providers }}</div>
        <div class="kpi-sub">JWK providers</div>
      </article>
    </section>

    <section class="dashboard-grid">
      <article class="panel">
        <h3>Service Flags</h3>
        <div class="flag-list">
          <div class="flag-row">
            <span>Root Auth</span>
            <n-tag size="small" :type="stats.root_auth_enabled ? 'success' : 'warning'">
              {{ stats.root_auth_enabled ? "enabled" : "disabled" }}
            </n-tag>
          </div>
          <div class="flag-row">
            <span>Root CORS</span>
            <n-tag size="small" :type="stats.root_cors_enabled ? 'success' : 'warning'">
              {{ stats.root_cors_enabled ? "enabled" : "disabled" }}
            </n-tag>
          </div>
          <div class="flag-row">
            <span>Admin Users Ratio</span>
            <span>{{ userAdminPercent }}%</span>
          </div>
        </div>
      </article>

      <article class="panel">
        <h3>Endpoint Methods</h3>
        <div class="method-bars">
          <div v-for="method in topMethods" :key="method.method" class="method-row">
            <div class="method-head">
              <strong>{{ method.method }}</strong>
              <span>{{ method.active }}/{{ method.total }}</span>
            </div>
            <div class="bar-track">
              <div class="bar-fill" :style="{ width: `${Math.round((method.total / topMethodTotal) * 100)}%` }"></div>
            </div>
          </div>
          <p v-if="topMethods.length === 0" class="muted">No endpoint methods yet.</p>
        </div>
      </article>
    </section>

    <section class="panel gateway-panel">
      <h3>Gateways</h3>
      <div class="table-wrap">
        <table class="data-table gateway-table">
          <thead>
            <tr>
              <th>Gateway</th>
              <th>Status</th>
              <th>Current Applied</th>
              <th>Apply Age</th>
              <th>Resources</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="gateway in gateways"
              :key="gateway.gateway_id"
              class="gateway-row-link"
              tabindex="0"
              @click="openGatewayDetails(gateway.gateway_id)"
              @keydown.enter.prevent="openGatewayDetails(gateway.gateway_id)"
              @keydown.space.prevent="openGatewayDetails(gateway.gateway_id)"
            >
              <td :title="gateway.host_name || ''">
                <div class="gateway-primary">{{ gateway.gateway_id }}</div>
                <div class="gateway-secondary">{{ gateway.host_name || "n/a" }}</div>
              </td>
              <td>
                <n-tag size="small" :type="gateway.status === 'online' ? 'success' : 'warning'">
                  {{ gateway.status }}
                </n-tag>
              </td>
              <td :title="`current: ${shortVersion(currentSnapshotVersion)} / gateway: ${shortVersion(gateway.snapshot_version)}`">
                <n-tag size="small" :type="isCurrentVersionApplied(gateway) ? 'success' : 'warning'">
                  {{ isCurrentVersionApplied(gateway) ? "yes" : "no" }}
                </n-tag>
              </td>
              <td :title="formatUnixTime(gateway.last_apply_at_unix)">{{ formatApplyAge(gateway.last_apply_at_unix) }}</td>
              <td class="gateway-resources-cell">
                <div class="gateway-resources-main">{{ formatMemoryBytes(gateway.memory_alloc_bytes) }}</div>
                <div class="gateway-resources-sub">{{ gateway.goroutines_count }} go</div>
              </td>
            </tr>
            <tr v-if="gateways.length === 0">
              <td colspan="5" class="muted">No gateways reported yet.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </template>
</template>
