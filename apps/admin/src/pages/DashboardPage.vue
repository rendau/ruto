<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { getStats } from "../lib/api";
import { notifyError } from "../lib/notify";
import type { StatsResponse } from "../types/api";

const loading = ref(false);
const stats = ref<StatsResponse | null>(null);
const errorMessage = ref("");

function safeNum(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
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
    stats.value = await getStats();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load dashboard stats";
    notifyError(errorMessage.value);
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void loadStats();
});
</script>

<template>
  <div class="actions">
    <button class="secondary-button" :disabled="loading" @click="loadStats">
      {{ loading ? "Refreshing..." : "Refresh" }}
    </button>
  </div>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
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
            <span class="status-chip" :class="{ inactive: !stats.root_auth_enabled }">
              {{ stats.root_auth_enabled ? "enabled" : "disabled" }}
            </span>
          </div>
          <div class="flag-row">
            <span>Root CORS</span>
            <span class="status-chip" :class="{ inactive: !stats.root_cors_enabled }">
              {{ stats.root_cors_enabled ? "enabled" : "disabled" }}
            </span>
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
  </template>
</template>
