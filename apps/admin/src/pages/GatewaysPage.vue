<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { getSnapshotVersion, listGateways } from "../lib/api";
import { formatUnixAge, formatUnixDateTime } from "../lib/datetime";
import { notifyError } from "../lib/notify";
import type { GatewayStateItem } from "../types/api";

const router = useRouter();
const gateways = ref<GatewayStateItem[]>([]);
const currentSnapshotVersion = ref("");
const loading = ref(false);
const errorMessage = ref("");

const onlineCount = computed(() => gateways.value.filter((gateway) => gateway.status === "online").length);
const staleCount = computed(() => gateways.value.filter((gateway) => gateway.status === "stale").length);
const offlineCount = computed(() => gateways.value.filter((gateway) => gateway.status === "offline").length);

async function loadGateways() {
  loading.value = true;
  errorMessage.value = "";
  try {
    const [gatewaysRep, snapshotVersionRep] = await Promise.all([listGateways(), getSnapshotVersion()]);
    gateways.value = gatewaysRep.results || [];
    currentSnapshotVersion.value = snapshotVersionRep.version || "";
  } catch (error) {
    gateways.value = [];
    errorMessage.value = error instanceof Error ? error.message : "Unable to load gateways";
    notifyError(errorMessage.value);
  } finally {
    loading.value = false;
  }
}

function isCurrentVersionApplied(gateway: GatewayStateItem): boolean {
  const currentVersion = (currentSnapshotVersion.value || "").trim();
  const gatewayVersion = (gateway.snapshot_version || "").trim();
  if (!currentVersion || !gatewayVersion) {
    return false;
  }
  return currentVersion === gatewayVersion;
}

function formatUnixTime(value: unknown): string {
  return formatUnixDateTime(value, "n/a");
}

function formatUnixAgeValue(value: unknown): string {
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

function openGatewayDetails(gatewayId: string): void {
  void router.push({ name: "gateway-details", params: { id: gatewayId } });
}

onMounted(() => {
  void loadGateways();
});
</script>

<template>
  <div class="actions page-top-actions gateways-top-actions">
    <button
      class="icon-action-button secondary"
      type="button"
      :disabled="loading"
      title="Refresh Gateways"
      aria-label="Refresh Gateways"
      @click="loadGateways"
    >
      <span class="icon-action-glyph">{{ loading ? "…" : "↻" }}</span>
    </button>
  </div>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>

  <section class="summary-grid gateways-summary-grid">
    <div>
      <span class="label">Total</span>
      <strong>{{ gateways.length }}</strong>
    </div>
    <div>
      <span class="label">Online</span>
      <strong>{{ onlineCount }}</strong>
    </div>
    <div>
      <span class="label">Stale</span>
      <strong>{{ staleCount }}</strong>
    </div>
    <div>
      <span class="label">Offline</span>
      <strong>{{ offlineCount }}</strong>
    </div>
  </section>

  <section class="panel">
    <h3>Gateways</h3>
    <div class="table-wrap">
      <table class="data-table gateway-table">
        <thead>
          <tr>
            <th>Gateway</th>
            <th>Status</th>
            <th>Current Applied</th>
            <th>Last Seen</th>
            <th>Last Apply</th>
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
              <span class="status-chip" :class="{ inactive: gateway.status !== 'online' }">
                {{ gateway.status }}
              </span>
            </td>
            <td>
              <span class="status-chip" :class="{ inactive: !isCurrentVersionApplied(gateway) }">
                {{ isCurrentVersionApplied(gateway) ? "yes" : "no" }}
              </span>
            </td>
            <td :title="formatUnixTime(gateway.last_seen_at_unix)">{{ formatUnixAgeValue(gateway.last_seen_at_unix) }}</td>
            <td :title="formatUnixTime(gateway.last_apply_at_unix)">{{ formatUnixAgeValue(gateway.last_apply_at_unix) }}</td>
            <td class="gateway-resources-cell">
              <div class="gateway-resources-main">{{ formatMemoryBytes(gateway.memory_alloc_bytes) }}</div>
              <div class="gateway-resources-sub">{{ gateway.goroutines_count }} go</div>
            </td>
          </tr>
          <tr v-if="!loading && gateways.length === 0">
            <td colspan="6" class="muted">No gateways reported yet.</td>
          </tr>
          <tr v-if="loading && gateways.length === 0">
            <td colspan="6" class="muted">Loading gateways...</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
