<script setup lang="ts">
import { onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import type { AppMain } from "../types/api";
import { listApps } from "../lib/api";
import { useAppsStore } from "../stores/apps";

const loading = ref(false);
const errorMessage = ref("");
const apps = ref<AppMain[]>([]);

const appsStore = useAppsStore();

async function loadApps() {
  loading.value = true;
  errorMessage.value = "";
  try {
    const rep = await listApps({
      list_params: {
        page: 1,
        page_size: 100,
        sort: ["name"]
      }
    });
    apps.value = rep.results;
    await appsStore.loadMenuApps();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load apps";
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void loadApps();
});
</script>

<template>
  <div class="page-header">
    <h2>Applications</h2>
    <RouterLink class="primary-button" to="/apps/new">Create App</RouterLink>
  </div>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
  <p v-if="loading" class="muted">Loading...</p>

  <table v-else class="data-table">
    <thead>
      <tr>
        <th>Name</th>
        <th>Path Prefix</th>
        <th>Status</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="item in apps" :key="item.id">
        <td>{{ item.name || item.id }}</td>
        <td>{{ item.path_prefix }}</td>
        <td>
          <span class="status-chip" :class="{ inactive: !item.active }">
            {{ item.active ? "active" : "inactive" }}
          </span>
        </td>
        <td class="actions">
          <RouterLink class="link-button" :to="{ name: 'app-details', params: { id: item.id } }">Open</RouterLink>
          <RouterLink class="link-button" :to="{ name: 'app-edit', params: { id: item.id } }">Edit</RouterLink>
        </td>
      </tr>
    </tbody>
  </table>
</template>
