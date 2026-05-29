<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { getSnapshotVersion, listGateways } from "../lib/api";
import { formatUnixAge, formatUnixDateTime } from "../lib/datetime";
import { notifyError, notifySuccess } from "../lib/notify";
import type { GatewayStateItem } from "../types/api";

const route = useRoute();
const router = useRouter();

const gateway = ref<GatewayStateItem | null>(null);
const currentSnapshotVersion = ref("");
const loading = ref(false);
const errorMessage = ref("");

const gatewayId = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));
const isCurrentVersionApplied = computed(() => {
  if (!gateway.value) {
    return false;
  }
  const currentVersion = (currentSnapshotVersion.value || "").trim();
  const gatewayVersion = (gateway.value.snapshot_version || "").trim();
  if (!currentVersion || !gatewayVersion) {
    return false;
  }
  return currentVersion === gatewayVersion;
});

async function loadGateway() {
  loading.value = true;
  errorMessage.value = "";
  try {
    const [gatewaysRep, snapshotVersionRep] = await Promise.all([listGateways(), getSnapshotVersion()]);
    currentSnapshotVersion.value = snapshotVersionRep.version || "";
    const found = (gatewaysRep.results || []).find((item) => item.gateway_id === gatewayId.value);
    if (!found) {
      gateway.value = null;
      errorMessage.value = `Gateway "${gatewayId.value}" not found`;
      return;
    }
    gateway.value = found;
  } catch (error) {
    gateway.value = null;
    errorMessage.value = error instanceof Error ? error.message : "Unable to load gateway details";
    notifyError(errorMessage.value);
  } finally {
    loading.value = false;
  }
}

function formatUnixTime(value: unknown): string {
  return formatUnixDateTime(value, "n/a");
}

function formatUnixAgeValue(value: unknown): string {
  return formatUnixAge(value, "n/a");
}

async function copySnapshotVersion(value: string): Promise<void> {
  const trimmed = (value || "").trim();
  if (!trimmed) {
    return;
  }
  try {
    await navigator.clipboard.writeText(trimmed);
    notifySuccess("Snapshot version copied");
  } catch {
    notifyError("Unable to copy snapshot version");
  }
}

function goBack(): void {
  if (window.history.length > 1) {
    router.back();
    return;
  }
  void router.push({ name: "dashboard" });
}

onMounted(() => {
  void loadGateway();
});
</script>

<template>
  <div class="actions page-top-actions gateway-details-actions">
    <button class="icon-action-button secondary" type="button" title="Back" aria-label="Back" @click="goBack">
      <svg class="icon-action-svg" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M15 18l-6-6 6-6" />
      </svg>
    </button>
    <button
      class="icon-action-button secondary"
      type="button"
      :disabled="loading"
      title="Refresh Gateway"
      aria-label="Refresh Gateway"
      @click="loadGateway"
    >
      <span class="icon-action-glyph">{{ loading ? "…" : "↻" }}</span>
    </button>
  </div>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
  <p v-if="loading && !gateway" class="muted">Loading gateway details...</p>

  <template v-if="gateway">
    <section class="summary-grid gateway-details-summary">
      <div>
        <span class="label">Gateway ID</span>
        <strong>{{ gateway.gateway_id }}</strong>
      </div>
      <div>
        <span class="label">Status</span>
        <span class="status-chip" :class="{ inactive: gateway.status !== 'online' }">
          {{ gateway.status }}
        </span>
      </div>
      <div>
        <span class="label">Current Applied</span>
        <span class="status-chip" :class="{ inactive: !isCurrentVersionApplied }">
          {{ isCurrentVersionApplied ? "yes" : "no" }}
        </span>
      </div>
      <div>
        <span class="label">Runtime</span>
        <strong>{{ gateway.host_name || "n/a" }}</strong>
      </div>
    </section>

    <section class="panel gateway-details-panel">
      <h3>Gateway Details</h3>
      <div class="gateway-details-grid">
        <div>
          <span class="label">Host Name</span>
          <strong>{{ gateway.host_name || "n/a" }}</strong>
        </div>
        <div>
          <span class="label">Snapshot Version</span>
          <button
            class="gateway-copy-link"
            type="button"
            :disabled="!gateway.snapshot_version"
            @click="copySnapshotVersion(gateway.snapshot_version)"
          >
            <span class="gateway-copy-value">{{ gateway.snapshot_version || "n/a" }}</span>
          </button>
        </div>
        <div>
          <span class="label">Started At</span>
          <strong>{{ formatUnixTime(gateway.started_at_unix) }}</strong>
        </div>
        <div>
          <span class="label">Started</span>
          <strong>{{ formatUnixAgeValue(gateway.started_at_unix) }}</strong>
        </div>
        <div>
          <span class="label">Last Seen At</span>
          <strong>{{ formatUnixTime(gateway.last_seen_at_unix) }}</strong>
        </div>
        <div>
          <span class="label">Last Seen</span>
          <strong>{{ formatUnixAgeValue(gateway.last_seen_at_unix) }}</strong>
        </div>
        <div>
          <span class="label">Last Apply At</span>
          <strong>{{ formatUnixTime(gateway.last_apply_at_unix) }}</strong>
        </div>
        <div>
          <span class="label">Last Apply</span>
          <strong>{{ formatUnixAgeValue(gateway.last_apply_at_unix) }}</strong>
        </div>
      </div>

      <div class="gateway-last-error">
        <span class="label">Last Error</span>
        <pre>{{ gateway.last_error || "none" }}</pre>
      </div>
    </section>
  </template>
</template>
