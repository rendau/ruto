<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { deleteApp, deleteEndpoint, getApp, listEndpoints, updateApp } from "../lib/api";
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
const togglingApp = ref(false);
const deletingEndpointId = ref("");
const endpointSearch = ref("");
const authVisibilityFilter = ref<"all" | "public" | "protected">("all");
const activeFilter = ref<"all" | "active" | "inactive">("all");
const httpMethodFilter = ref("all");

const app = ref<AppMain | null>(null);
const endpoints = ref<EndpointMain[]>([]);

type EndpointAuthIcon = {
  key: "ip_validation" | "jwt" | "basic" | "api_key";
  glyph: string;
  label: string;
};

const methodSortOrder: Record<string, number> = {
  GET: 1,
  POST: 2,
  PUT: 3,
  PATCH: 4,
  DELETE: 5,
  OPTIONS: 6,
  HEAD: 7
};

const httpMethodOptions = computed(() => {
  const items = new Set<string>();
  for (const endpoint of endpoints.value) {
    const method = (endpoint.method || "").trim().toUpperCase();
    if (method) {
      items.add(method);
    }
  }
  return Array.from(items).sort((a, b) => {
    const orderA = methodSortOrder[a] || 99;
    const orderB = methodSortOrder[b] || 99;
    if (orderA !== orderB) {
      return orderA - orderB;
    }
    return a.localeCompare(b);
  });
});

const filteredEndpoints = computed(() => {
  const query = endpointSearch.value.trim().toLowerCase();
  return endpoints.value.filter((endpoint) => {
    const path = normalizedRoutePath(endpoint.path);
    const method = (endpoint.method || "").trim().toUpperCase();
    const requiresAuth = endpointRequiresAuth(endpoint);
    const isActive = Boolean(endpoint.active);

    if (query) {
      const target = `${method} ${path}`.toLowerCase();
      if (!target.includes(query)) {
        return false;
      }
    }

    if (authVisibilityFilter.value === "public" && requiresAuth) {
      return false;
    }
    if (authVisibilityFilter.value === "protected" && !requiresAuth) {
      return false;
    }

    if (activeFilter.value === "active" && !isActive) {
      return false;
    }
    if (activeFilter.value === "inactive" && isActive) {
      return false;
    }

    if (httpMethodFilter.value !== "all" && method !== httpMethodFilter.value) {
      return false;
    }

    return true;
  });
});

const endpointGroups = computed(() => {
  const groups = new Map<string, EndpointMain[]>();
  for (const endpoint of filteredEndpoints.value) {
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

const hasEndpointFilters = computed(() => {
  return (
    endpointSearch.value.trim() !== "" ||
    authVisibilityFilter.value !== "all" ||
    activeFilter.value !== "all" ||
    httpMethodFilter.value !== "all"
  );
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

function resetEndpointFilters() {
  endpointSearch.value = "";
  authVisibilityFilter.value = "all";
  activeFilter.value = "all";
  httpMethodFilter.value = "all";
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
  if (deletingApp.value || togglingApp.value) {
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

async function toggleAppActive() {
  if (!app.value || deletingApp.value || togglingApp.value) {
    return;
  }

  const nextActive = !app.value.active;
  const action = nextActive ? "Activate" : "Deactivate";
  const approved = window.confirm(`${action} application "${app.value.name || app.value.id}"?`);
  if (!approved) {
    return;
  }

  togglingApp.value = true;
  errorMessage.value = "";
  try {
    await updateApp({
      ...app.value,
      active: nextActive
    });
    app.value.active = nextActive;
    await appsStore.loadMenuApps();
    notifySuccess(`Application ${nextActive ? "activated" : "deactivated"}`);
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to update app status";
    notifyError(errorMessage.value);
  } finally {
    togglingApp.value = false;
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="actions page-top-actions">
    <RouterLink class="primary-button" :to="{ name: 'endpoint-create', params: { appId: id } }">Create Endpoint</RouterLink>
    <button
      v-if="app"
      :class="app.active ? 'danger-button' : 'primary-button'"
      :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
      :title="app.active ? 'Deactivate App' : 'Activate App'"
      @click="toggleAppActive"
    >
      {{ togglingApp ? "Saving..." : app.active ? "Deactivate App" : "Activate App" }}
    </button>
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
      :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
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

    <div class="page-header compact endpoints-head">
      <h3>Endpoints</h3>
      <span class="muted endpoints-head-count">{{ filteredEndpoints.length }} / {{ endpoints.length }}</span>
    </div>
    <div class="endpoints-filters">
      <input
        v-model="endpointSearch"
        class="endpoints-filter-input"
        type="search"
        placeholder="Search endpoints"
        aria-label="Search endpoints"
      />
      <select v-model="authVisibilityFilter" class="endpoints-filter-select" aria-label="Filter by visibility">
        <option value="all">All Visibility</option>
        <option value="public">Public</option>
        <option value="protected">Protected</option>
      </select>
      <select v-model="activeFilter" class="endpoints-filter-select" aria-label="Filter by status">
        <option value="all">All Status</option>
        <option value="active">Active</option>
        <option value="inactive">Inactive</option>
      </select>
      <select v-model="httpMethodFilter" class="endpoints-filter-select" aria-label="Filter by HTTP method">
        <option value="all">All Methods</option>
        <option v-for="method in httpMethodOptions" :key="method" :value="method">{{ method }}</option>
      </select>
      <button
        class="icon-action-button secondary"
        type="button"
        :disabled="!hasEndpointFilters"
        title="Reset Filters"
        aria-label="Reset Filters"
        @click="resetEndpointFilters"
      >
        <span class="icon-action-glyph">✖</span>
      </button>
    </div>
    <table class="data-table endpoints-table">
      <tbody v-if="endpointGroups.length === 0">
        <tr>
          <td colspan="5" class="muted">{{ hasEndpointFilters ? "No endpoints match filters." : "No endpoints found." }}</td>
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
          <td class="endpoint-path-cell">
            <RouterLink
              class="endpoint-path-link"
              :to="{ name: 'endpoint-details', params: { id: endpoint.id } }"
              :title="`${endpoint.method} ${normalizedRoutePath(endpoint.path)}`"
            >
              {{ normalizedRoutePath(endpoint.path) }}
            </RouterLink>
          </td>
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
              :to="{ name: 'endpoint-details', params: { id: endpoint.id } }"
              title="View Endpoint"
              aria-label="View Endpoint"
            >
              <span class="icon-action-glyph">◉</span>
            </RouterLink>
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
              :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
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

    <div class="endpoints-mobile-list">
      <p v-if="endpointGroups.length === 0" class="muted endpoints-mobile-empty">
        {{ hasEndpointFilters ? "No endpoints match filters." : "No endpoints found." }}
      </p>
      <section v-for="group in endpointGroups" :key="`mobile-${group.segment}`" class="endpoint-mobile-group">
        <div class="endpoint-group-head">
          <span class="endpoint-group-segment">{{ group.segment }}</span>
          <span class="endpoint-group-count">{{ group.items.length }}</span>
        </div>
        <div class="endpoint-mobile-cards">
          <article v-for="endpoint in group.items" :key="`mobile-${endpoint.id}`" class="endpoint-mobile-card">
            <div class="endpoint-mobile-top">
              <span class="http-method-badge" :class="endpointMethodBadgeClass(endpoint.method)">{{ endpoint.method }}</span>
              <span class="status-chip" :class="{ inactive: !endpoint.active }">
                {{ endpoint.active ? "active" : "inactive" }}
              </span>
            </div>
            <RouterLink
              class="endpoint-path-link endpoint-mobile-path"
              :to="{ name: 'endpoint-details', params: { id: endpoint.id } }"
              :title="`${endpoint.method} ${normalizedRoutePath(endpoint.path)}`"
            >
              {{ normalizedRoutePath(endpoint.path) }}
            </RouterLink>
            <div class="endpoint-auth endpoint-mobile-auth">
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
                  :key="`mobile-${endpoint.id}-${authIcon.key}`"
                  class="endpoint-auth-icon"
                  :title="authIcon.label"
                  :aria-label="authIcon.label"
                >
                  {{ authIcon.glyph }}
                </span>
              </div>
            </div>
            <div class="endpoint-mobile-actions">
              <RouterLink
                class="icon-action-button secondary"
                :to="{ name: 'endpoint-details', params: { id: endpoint.id } }"
                title="View Endpoint"
                aria-label="View Endpoint"
              >
                <span class="icon-action-glyph">◉</span>
              </RouterLink>
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
                :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
                title="Delete Endpoint"
                aria-label="Delete Endpoint"
                @click="removeEndpoint(endpoint)"
              >
                <span class="icon-action-glyph">{{ deletingEndpointId === endpoint.id ? "…" : "✕" }}</span>
              </button>
            </div>
          </article>
        </div>
      </section>
    </div>
  </template>
</template>
