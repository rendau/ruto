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

type EndpointAuthIcon = {
  key: "ip_validation" | "jwt" | "basic" | "api_key";
  glyph: string;
  label: string;
};

const endpointGroups = computed(() => {
  const groups = new Map<string, EndpointMain[]>();
  for (const endpoint of endpoints.value) {
    const key = firstPathSegment(endpoint.path);
    const current = groups.get(key);
    if (current) {
      current.push(endpoint);
    } else {
      groups.set(key, [endpoint]);
    }
  }

  return Array.from(groups.entries())
    .map(([segment, items]) => ({
      segment,
      items: [...items].sort((a, b) => sortEndpoints(a, b))
    }))
    .sort((a, b) => {
      if (a.segment === "/" && b.segment !== "/") {
        return -1;
      }
      if (a.segment !== "/" && b.segment === "/") {
        return 1;
      }
      return a.segment.localeCompare(b.segment);
    });
});

function firstPathSegment(path: string): string {
  const normalized = (path || "").trim();
  if (!normalized || normalized === "/") {
    return "/";
  }
  const parts = normalized.split("/").filter(Boolean);
  if (parts.length === 0) {
    return "/";
  }
  return `/${parts[0]}`;
}

function sortEndpoints(a: EndpointMain, b: EndpointMain): number {
  const pathCompare = normalizedRoutePath(a.path).localeCompare(normalizedRoutePath(b.path));
  if (pathCompare !== 0) {
    return pathCompare;
  }
  return (a.method || "").localeCompare(b.method || "");
}

function normalizedRoutePath(path: string): string {
  const trimmed = (path || "").trim();
  if (!trimmed) {
    return "/";
  }
  return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
}

function endpointAuthIcons(endpoint: EndpointMain): EndpointAuthIcon[] {
  const methods = endpoint.auth?.methods || [];
  const hasIpValidation = methods.some((item) => Boolean(item.ip_validation));
  const hasJwt = methods.some((item) => Boolean(item.jwt));
  const hasBasic = methods.some((item) => Boolean(item.basic));
  const hasApiKey = methods.some((item) => Boolean(item.api_key));

  const icons: EndpointAuthIcon[] = [];
  if (hasIpValidation) {
    icons.push({ key: "ip_validation", glyph: "IP", label: "IP Validation" });
  }
  if (hasJwt) {
    icons.push({ key: "jwt", glyph: "JWT", label: "JWT" });
  }
  if (hasBasic) {
    icons.push({ key: "basic", glyph: "B", label: "Basic Auth" });
  }
  if (hasApiKey) {
    icons.push({ key: "api_key", glyph: "K", label: "API Key" });
  }
  return icons;
}

function endpointRequiresAuth(endpoint: EndpointMain): boolean {
  return Boolean(endpoint.auth?.enabled);
}

function endpointMethodBadgeClass(method: string): string {
  const normalized = (method || "").trim().toUpperCase();
  switch (normalized) {
    case "GET":
      return "method-get";
    case "POST":
      return "method-post";
    case "PUT":
      return "method-put";
    case "DELETE":
      return "method-delete";
    case "PATCH":
      return "method-patch";
    case "HEAD":
      return "method-head";
    case "OPTIONS":
      return "method-options";
    default:
      return "method-other";
  }
}

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
  <div class="actions page-top-actions">
    <RouterLink class="primary-button" :to="{ name: 'endpoint-create', params: { appId: id } }">Create Endpoint</RouterLink>
    <RouterLink
      class="icon-action-button secondary"
      :to="{ name: 'app-edit', params: { id } }"
      title="Edit App"
      aria-label="Edit App"
    >
      <span class="icon-action-glyph">✎</span>
    </RouterLink>
    <button
      class="icon-action-button danger"
      :disabled="deletingApp || deletingEndpointId !== ''"
      title="Delete App"
      aria-label="Delete App"
      @click="removeApp"
    >
      <span class="icon-action-glyph">✕</span>
    </button>
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
    <table class="data-table endpoints-table">
      <tbody v-if="endpointGroups.length === 0">
        <tr>
          <td colspan="5" class="muted">No endpoints found.</td>
        </tr>
      </tbody>

      <tbody v-for="(group, groupIndex) in endpointGroups" :key="group.segment">
        <tr v-if="groupIndex > 0" class="endpoint-group-spacer" aria-hidden="true">
          <td colspan="5"></td>
        </tr>
        <tr class="endpoint-group-row">
          <td colspan="5">
            <div class="endpoint-group-head">
              <span class="endpoint-group-segment">{{ group.segment }}</span>
              <span class="endpoint-group-count">{{ group.items.length }}</span>
            </div>
          </td>
        </tr>
        <tr v-for="endpoint in group.items" :key="endpoint.id" class="endpoint-route-row">
          <td>
            <span class="http-method-badge" :class="endpointMethodBadgeClass(endpoint.method)">{{ endpoint.method }}</span>
          </td>
          <td class="endpoint-path-cell">{{ normalizedRoutePath(endpoint.path) }}</td>
          <td>
            <div class="endpoint-auth">
              <span
                v-if="endpointRequiresAuth(endpoint)"
                class="endpoint-lock-chip"
                title="Auth required"
                aria-label="Auth required"
              >
                🔒
              </span>
              <div v-if="endpointAuthIcons(endpoint).length > 0" class="endpoint-auth-icons">
                <span
                  v-for="authIcon in endpointAuthIcons(endpoint)"
                  :key="authIcon.key"
                  class="endpoint-auth-icon"
                  :title="authIcon.label"
                  :aria-label="authIcon.label"
                >
                  {{ authIcon.glyph }}
                </span>
              </div>
            </div>
          </td>
          <td>
            <span class="status-chip" :class="{ inactive: !endpoint.active }">
              {{ endpoint.active ? "active" : "inactive" }}
            </span>
          </td>
          <td class="actions">
            <RouterLink
              class="icon-action-button secondary"
              :to="{ name: 'endpoint-edit', params: { id: endpoint.id } }"
              title="Edit Endpoint"
              aria-label="Edit Endpoint"
            >
              <span class="icon-action-glyph">✎</span>
            </RouterLink>
            <button
              class="icon-action-button danger"
              :disabled="deletingApp || deletingEndpointId !== ''"
              title="Delete Endpoint"
              aria-label="Delete Endpoint"
              @click="removeEndpoint(endpoint)"
            >
              <span class="icon-action-glyph">{{ deletingEndpointId === endpoint.id ? "…" : "✕" }}</span>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </template>
</template>
