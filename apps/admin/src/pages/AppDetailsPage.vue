<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave, RouterLink, useRoute, useRouter, type RouteLocationNormalizedLoaded } from "vue-router";
import { deleteApp, deleteEndpoint, getApp, getAppSwaggerEndpointsDiff, listEndpoints, updateApp } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppMain, AppSwaggerEndpoint, EndpointMain } from "../types/api";
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
const swaggerUnregistered = ref<AppSwaggerEndpoint[]>([]);
const swaggerRegisteredInvalid = ref<AppSwaggerEndpoint[]>([]);
const swaggerDiffLoading = ref(false);
const swaggerDiffError = ref("");
const swaggerPanelOpen = ref(false);
const swaggerDiffLoaded = ref(false);

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

type SwaggerEndpointGroup = {
  segment: string;
  items: AppSwaggerEndpoint[];
};

function sortSwaggerEndpoints(a: AppSwaggerEndpoint, b: AppSwaggerEndpoint): number {
  const pathCompare = normalizedRoutePath(a.path).localeCompare(normalizedRoutePath(b.path));
  if (pathCompare !== 0) {
    return pathCompare;
  }
  return (a.method || "").localeCompare(b.method || "");
}

function groupSwaggerEndpoints(items: AppSwaggerEndpoint[]): SwaggerEndpointGroup[] {
  const groups = new Map<string, AppSwaggerEndpoint[]>();
  for (const item of items) {
    const key = firstPathSegment(item.path);
    const current = groups.get(key);
    if (current) {
      current.push(item);
    } else {
      groups.set(key, [item]);
    }
  }

  return Array.from(groups.entries())
    .map(([segment, groupItems]) => ({
      segment,
      items: [...groupItems].sort((a, b) => sortSwaggerEndpoints(a, b))
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
}

const swaggerUnregisteredGroups = computed(() => groupSwaggerEndpoints(swaggerUnregistered.value));
const swaggerRegisteredInvalidGroups = computed(() => groupSwaggerEndpoints(swaggerRegisteredInvalid.value));
const swaggerUnregisteredMethodsByPath = computed(() => {
  const result = new Map<string, string[]>();
  for (const item of swaggerUnregistered.value) {
    const path = normalizedRoutePath(item.path);
    const method = (item.method || "").trim().toUpperCase();
    if (!method) {
      continue;
    }
    const current = result.get(path);
    if (current) {
      if (!current.includes(method)) {
        current.push(method);
      }
    } else {
      result.set(path, [method]);
    }
  }
  for (const methods of result.values()) {
    methods.sort((a, b) => a.localeCompare(b));
  }
  return result;
});

function registeredInvalidReason(item: AppSwaggerEndpoint): string {
  const path = normalizedRoutePath(item.path);
  const swaggerMethods = swaggerUnregisteredMethodsByPath.value.get(path) || [];
  if (swaggerMethods.length > 0) {
    return "Неверно зарегистрирован";
  }
  return "Отсутствует в Swagger";
}

const hasEndpointFilters = computed(() => {
  return (
    endpointSearch.value.trim() !== "" ||
    authVisibilityFilter.value !== "all" ||
    activeFilter.value !== "all" ||
    httpMethodFilter.value !== "all"
  );
});

type SavedEndpointFilters = {
  endpoint_search?: string;
  auth_visibility_filter?: "all" | "public" | "protected";
  active_filter?: "all" | "active" | "inactive";
  http_method_filter?: string;
};

function endpointFiltersStorageKey(): string {
  return `app-details:endpoint-filters:${id.value || "_"}`;
}

function restoreEndpointFilters() {
  const raw = window.sessionStorage.getItem(endpointFiltersStorageKey());
  if (!raw) {
    return;
  }

  try {
    const parsed = JSON.parse(raw) as SavedEndpointFilters;
    endpointSearch.value = typeof parsed.endpoint_search === "string" ? parsed.endpoint_search : "";
    authVisibilityFilter.value =
      parsed.auth_visibility_filter === "public" || parsed.auth_visibility_filter === "protected"
        ? parsed.auth_visibility_filter
        : "all";
    activeFilter.value =
      parsed.active_filter === "active" || parsed.active_filter === "inactive" ? parsed.active_filter : "all";
    httpMethodFilter.value = typeof parsed.http_method_filter === "string" ? parsed.http_method_filter : "all";
  } catch {
    endpointSearch.value = "";
    authVisibilityFilter.value = "all";
    activeFilter.value = "all";
    httpMethodFilter.value = "all";
  }
}

function persistEndpointFilters() {
  const payload: SavedEndpointFilters = {
    endpoint_search: endpointSearch.value,
    auth_visibility_filter: authVisibilityFilter.value,
    active_filter: activeFilter.value,
    http_method_filter: httpMethodFilter.value
  };
  window.sessionStorage.setItem(endpointFiltersStorageKey(), JSON.stringify(payload));
}

function clearPersistedEndpointFilters() {
  window.sessionStorage.removeItem(endpointFiltersStorageKey());
}

function isWithinCurrentAppContext(to: RouteLocationNormalizedLoaded): boolean {
  const name = typeof to.name === "string" ? to.name : "";
  const currentAppId = id.value;

  if (name === "app-details" || name === "app-edit") {
    return typeof to.params.id === "string" && to.params.id === currentAppId;
  }
  if (name === "endpoint-create") {
    return typeof to.params.appId === "string" && to.params.appId === currentAppId;
  }
  if (name === "endpoint-details" || name === "endpoint-edit") {
    return true;
  }

  return false;
}

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
  swaggerDiffError.value = "";
  swaggerUnregistered.value = [];
  swaggerRegisteredInvalid.value = [];
  swaggerPanelOpen.value = false;
  swaggerDiffLoaded.value = false;
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
    swaggerDiffLoading.value = false;
  }
}

async function loadSwaggerDiff() {
  if (!app.value || !app.value.backend.swagger_url || swaggerDiffLoading.value || swaggerDiffLoaded.value) {
    return;
  }

  swaggerDiffLoading.value = true;
  swaggerDiffError.value = "";
  try {
    const diffRep = await getAppSwaggerEndpointsDiff(id.value);
    swaggerUnregistered.value = diffRep.unregistered || [];
    swaggerRegisteredInvalid.value = diffRep.registered_invalid || [];
    swaggerDiffLoaded.value = true;
  } catch (error) {
    swaggerDiffError.value = error instanceof Error ? error.message : "Unable to load swagger endpoints";
  } finally {
    swaggerDiffLoading.value = false;
  }
}

function toggleSwaggerPanel() {
  if (!app.value?.backend.swagger_url) {
    return;
  }
  swaggerPanelOpen.value = !swaggerPanelOpen.value;
  if (swaggerPanelOpen.value) {
    void loadSwaggerDiff();
  }
}

function closeSwaggerPanel() {
  swaggerPanelOpen.value = false;
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
  restoreEndpointFilters();
  void load();
});

watch([id, endpointSearch, authVisibilityFilter, activeFilter, httpMethodFilter], () => {
  persistEndpointFilters();
});

onBeforeRouteLeave((to) => {
  if (!isWithinCurrentAppContext(to)) {
    clearPersistedEndpointFilters();
  }
});
</script>

<template>
  <div class="actions page-top-actions">
    <RouterLink class="primary-button" :to="{ name: 'endpoint-create', params: { appId: id } }">Create Endpoint</RouterLink>
    <button
      v-if="app?.backend.swagger_url"
      class="secondary-button swagger-toggle-button"
      :disabled="loading || deletingApp || deletingEndpointId !== '' || togglingApp"
      @click="toggleSwaggerPanel"
    >
      <svg class="icon-action-svg swagger-toggle-icon" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M9 4H5a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h4" />
        <path d="M15 4h4a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1h-4" />
        <path d="M10 8h4" />
        <path d="M10 12h4" />
        <path d="M10 16h4" />
      </svg>
      <span>Swagger</span>
    </button>
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

    <Transition name="swagger-panel">
      <section v-if="app.backend.swagger_url && swaggerPanelOpen" class="panel swagger-diff-panel">
        <button
          class="icon-action-button secondary swagger-close-icon"
          type="button"
          title="Close Swagger Diff"
          aria-label="Close Swagger Diff"
          @click="closeSwaggerPanel"
        >
          <span class="icon-action-glyph">✕</span>
        </button>
        <div class="page-header endpoints-head swagger-panel-head">
          <h3>Swagger Diff</h3>
        </div>
        <p v-if="swaggerDiffLoading" class="muted">Loading swagger endpoints...</p>
        <p v-else-if="swaggerDiffError" class="error">{{ swaggerDiffError }}</p>
        <template v-else>
          <div class="swagger-diff-grid">
            <div class="swagger-diff-column">
              <div class="swagger-diff-title-row">
                <div class="label">Not Registered</div>
                <span class="swagger-diff-count">{{ swaggerUnregistered.length }}</span>
              </div>
              <div v-if="swaggerUnregisteredGroups.length > 0" class="swagger-endpoint-groups">
                <section v-for="group in swaggerUnregisteredGroups" :key="`missing-group-${group.segment}`" class="swagger-endpoint-group">
                  <div class="endpoint-group-head">
                    <span class="endpoint-group-segment">{{ group.segment }}</span>
                    <span class="endpoint-group-count">{{ group.items.length }}</span>
                  </div>
                  <ul class="swagger-endpoint-list">
                    <li
                      v-for="item in group.items"
                      :key="`missing-${group.segment}-${item.method}-${item.path}`"
                      class="swagger-endpoint-item"
                    >
                      <span class="http-method-badge" :class="endpointMethodBadgeClass(item.method)">{{ item.method }}</span>
                      <span class="swagger-endpoint-path">{{ item.path }}</span>
                      <RouterLink
                        class="primary-button swagger-add-button"
                        :to="{
                          name: 'endpoint-create',
                          params: { appId: id },
                          query: { method: item.method, path: item.path }
                        }"
                        :aria-label="`Add endpoint ${item.method} ${item.path}`"
                      >
                        <span class="swagger-add-plus">+</span>
                        <span>Add</span>
                      </RouterLink>
                    </li>
                  </ul>
                </section>
              </div>
              <p v-else class="muted">No missing endpoints.</p>
            </div>
            <div class="swagger-diff-column">
              <div class="swagger-diff-title-row">
                <div class="label">Registered Invalid</div>
                <span class="swagger-diff-count">{{ swaggerRegisteredInvalid.length }}</span>
              </div>
              <div v-if="swaggerRegisteredInvalidGroups.length > 0" class="swagger-endpoint-groups">
                <section
                  v-for="group in swaggerRegisteredInvalidGroups"
                  :key="`invalid-group-${group.segment}`"
                  class="swagger-endpoint-group"
                >
                  <div class="endpoint-group-head">
                    <span class="endpoint-group-segment">{{ group.segment }}</span>
                    <span class="endpoint-group-count">{{ group.items.length }}</span>
                  </div>
                  <ul class="swagger-endpoint-list">
                    <li
                      v-for="item in group.items"
                      :key="`invalid-${group.segment}-${item.method}-${item.path}`"
                      class="swagger-endpoint-item swagger-endpoint-item-invalid"
                    >
                      <span class="http-method-badge" :class="endpointMethodBadgeClass(item.method)">{{ item.method }}</span>
                      <span class="swagger-endpoint-path">{{ item.path }}</span>
                      <span class="swagger-endpoint-reason">{{ registeredInvalidReason(item) }}</span>
                    </li>
                  </ul>
                </section>
              </div>
              <p v-else class="muted">No invalid registrations.</p>
            </div>
          </div>
        </template>
      </section>
    </Transition>

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
