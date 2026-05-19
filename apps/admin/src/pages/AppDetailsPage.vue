<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { deleteApp, deleteEndpoint, getApp, listEndpoints } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppMain, EndpointMain } from "../types/api";
import { useAppsStore } from "../stores/apps";

const route = useRoute();
const router = useRouter();
const appsStore = useAppsStore();

const id = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));
const loading = ref(false);
const errorMessage = ref("");
const deletingApp = ref(false);
const deletingEndpointId = ref("");

const app = ref<AppMain | null>(null);
const endpoints = ref<EndpointMain[]>([]);

async function load() {
  loading.value = true;
  errorMessage.value = "";
  try {
    app.value = await getApp(id.value);
    const endpointList = await listEndpoints({
      app_id: id.value
    });
    endpoints.value = endpointList.results;
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load application";
  } finally {
    loading.value = false;
  }
}

async function removeEndpoint(endpoint: EndpointMain) {
  if (deletingEndpointId.value) {
    return;
  }
  const approved = window.confirm(`Delete endpoint ${endpoint.method} ${endpoint.path}?`);
  if (!approved) {
    return;
  }
  deletingEndpointId.value = endpoint.id;
  try {
    await deleteEndpoint(endpoint.id);
    notifySuccess("Endpoint deleted");
    await load();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to delete endpoint";
    notifyError(errorMessage.value);
  } finally {
    deletingEndpointId.value = "";
  }
}

async function removeApp() {
  if (deletingApp.value) {
    return;
  }
  const approved = window.confirm(`Delete application "${app.value?.name || app.value?.id}"?`);
  if (!approved || !app.value) {
    return;
  }
  deletingApp.value = true;
  try {
    await deleteApp(app.value.id);
    await appsStore.loadMenuApps();
    notifySuccess("Application deleted");
    await router.push({ name: "dashboard" });
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to delete app";
    notifyError(errorMessage.value);
  } finally {
    deletingApp.value = false;
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="page-header">
    <h2>Application</h2>
    <div class="actions">
      <RouterLink class="primary-button" :to="{ name: 'endpoint-create', params: { appId: id } }">Create Endpoint</RouterLink>
      <RouterLink class="secondary-button" :to="{ name: 'app-edit', params: { id } }">Edit App</RouterLink>
      <button class="danger-button" :disabled="deletingApp || deletingEndpointId !== ''" @click="removeApp">
        {{ deletingApp ? "Deleting..." : "Delete App" }}
      </button>
    </div>
  </div>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
  <p v-if="loading" class="muted">Loading...</p>

  <template v-else-if="app">
    <section class="summary-grid">
      <div>
        <span class="label">Name</span>
        <strong>{{ app.name }}</strong>
      </div>
      <div>
        <span class="label">Path Prefix</span>
        <strong>{{ app.path_prefix }}</strong>
      </div>
      <div>
        <span class="label">Backend</span>
        <strong>{{ app.backend.url }}</strong>
      </div>
      <div>
        <span class="label">Status</span>
        <strong>{{ app.active ? "active" : "inactive" }}</strong>
      </div>
    </section>

    <div class="page-header compact">
      <h3>Endpoints</h3>
    </div>
    <table class="data-table">
      <thead>
        <tr>
          <th>Method</th>
          <th>Path</th>
          <th>Custom Path</th>
          <th>Status</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="endpoint in endpoints" :key="endpoint.id">
          <td>{{ endpoint.method }}</td>
          <td>{{ endpoint.path }}</td>
          <td>{{ endpoint.backend.custom_path || "-" }}</td>
          <td>
            <span class="status-chip" :class="{ inactive: !endpoint.active }">
              {{ endpoint.active ? "active" : "inactive" }}
            </span>
          </td>
          <td class="actions">
            <RouterLink class="link-button" :to="{ name: 'endpoint-edit', params: { id: endpoint.id } }">Edit</RouterLink>
            <button
              class="danger-text-button"
              :disabled="deletingApp || deletingEndpointId !== ''"
              @click="removeEndpoint(endpoint)"
            >
              {{ deletingEndpointId === endpoint.id ? "Deleting..." : "Delete" }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </template>
</template>
