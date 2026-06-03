<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type Component } from "vue";
import { onBeforeRouteLeave, RouterLink, useRoute, useRouter, type RouteLocationNormalizedLoaded } from "vue-router";
import { useDialog } from "naive-ui";
import {
  AddOutline,
  CreateOutline,
  DocumentTextOutline,
  GitCompareOutline,
  GlobeOutline,
  KeyOutline,
  LockClosedOutline,
  PersonOutline,
  RefreshOutline,
  SearchOutline,
  TerminalOutline,
  TrashOutline
} from "@vicons/ionicons5";
import {
  createEndpoint,
  deleteApp,
  deleteEndpoint,
  getApp,
  getAppGrpcReflectionEndpoints,
  getAppSwaggerEndpointsDiff,
  getRoot,
  listEndpoints,
  updateApp
} from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppGrpcReflectionEndpoint, AppMain, AppSwaggerEndpoint, EndpointMain, EndpointType, RootMain } from "../types/api";
import { useAppsStore } from "../stores/apps";
import GrpcInstructionPanel from "../components/GrpcInstructionPanel.vue";

const route = useRoute();
const router = useRouter();
const appsStore = useAppsStore();
const dialog = useDialog();

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
const activeEndpointType = ref<EndpointType>("http");

const app = ref<AppMain | null>(null);
const endpoints = ref<EndpointMain[]>([]);
const swaggerUnregistered = ref<AppSwaggerEndpoint[]>([]);
const swaggerRegisteredInvalid = ref<AppSwaggerEndpoint[]>([]);
const swaggerDiffLoading = ref(false);
const swaggerDiffError = ref("");
const swaggerPanelOpen = ref(false);
const swaggerDiffLoaded = ref(false);
const swaggerRegisteredInvalidOpen = ref(false);
const swaggerBulkAdding = ref(false);
const swaggerSelectedKeys = ref<Record<string, boolean>>({});
const swaggerDiffSearch = ref("");
const grpcReflectionEndpoints = ref<AppGrpcReflectionEndpoint[]>([]);
const grpcUnregistered = ref<AppGrpcReflectionEndpoint[]>([]);
const grpcRegisteredInvalid = ref<AppGrpcReflectionEndpoint[]>([]);
const grpcDiffLoading = ref(false);
const grpcDiffError = ref("");
const grpcPanelOpen = ref(false);
const grpcDiffLoaded = ref(false);
const grpcRegisteredInvalidOpen = ref(false);
const grpcBulkAdding = ref(false);
const grpcSelectedKeys = ref<Record<string, boolean>>({});
const grpcDiffSearch = ref("");
const grpcInstructionOpen = ref(false);
const root = ref<RootMain | null>(null);

type EndpointAuthIcon = {
  key: "ip_validation" | "jwt" | "basic" | "api_key";
  component: Component;
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

const protocolOptions = computed(() => {
  const options: Array<{ value: EndpointType; label: string }> = [{ value: "http", label: "HTTP" }];
  const hasGrpcPort = Number(app.value?.backend.grpc_port || 0) > 0;
  if (hasGrpcPort || grpcEndpoints.value.length > 0) {
    options.push({ value: "grpc", label: "gRPC" });
  }
  return options;
});
const httpEndpoints = computed(() => endpoints.value.filter((endpoint) => endpointType(endpoint) === "http"));
const grpcEndpoints = computed(() => endpoints.value.filter((endpoint) => endpointType(endpoint) === "grpc"));
const activeProtocolTotal = computed(() => (activeEndpointType.value === "grpc" ? grpcEndpoints.value.length : httpEndpoints.value.length));

const httpMethodOptions = computed(() => {
  const items = new Set<string>();
  for (const endpoint of httpEndpoints.value) {
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
const authVisibilityOptions = [
  { label: "All Visibility", value: "all" },
  { label: "Public", value: "public" },
  { label: "Protected", value: "protected" }
];
const activeFilterOptions = [
  { label: "All Status", value: "all" },
  { label: "Active", value: "active" },
  { label: "Inactive", value: "inactive" }
];
const httpMethodFilterOptions = computed(() => [
  { label: "All Methods", value: "all" },
  ...httpMethodOptions.value.map((method) => ({ label: method, value: method }))
]);

const filteredEndpoints = computed(() => {
  const query = endpointSearch.value.trim().toLowerCase();
  return endpoints.value.filter((endpoint) => {
    const type = endpointType(endpoint);
    const path = endpointRoutePath(endpoint);
    const method = endpointRouteMethod(endpoint);
    const requiresAuth = endpointRequiresAuth(endpoint);
    const isActive = Boolean(endpoint.active);

    if (type !== activeEndpointType.value) {
      return false;
    }

    if (query) {
      const target = `${method} ${path} ${endpoint.grpc?.service || ""} ${endpoint.grpc?.method || ""}`.toLowerCase();
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

    if (type === "http" && httpMethodFilter.value !== "all" && method !== httpMethodFilter.value) {
      return false;
    }

    return true;
  });
});

const endpointGroups = computed(() => {
  const groups = new Map<string, EndpointMain[]>();
  for (const endpoint of filteredEndpoints.value) {
    const key = endpointGroupKey(endpoint);
    const current = groups.get(key);
    if (current) {
      current.push(endpoint);
    } else {
      groups.set(key, [endpoint]);
    }
  }

  return Array.from(groups.entries())
    .map(([key, items]) => ({
      key,
      segment: endpointGroupSegment(items[0], key),
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
type GrpcEndpointGroup = {
  segment: string;
  items: AppGrpcReflectionEndpoint[];
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
function filterSwaggerGroups(groups: SwaggerEndpointGroup[], query: string): SwaggerEndpointGroup[] {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return groups;
  }
  return groups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => `${item.method} ${item.path}`.toLowerCase().includes(normalizedQuery))
    }))
    .filter((group) => group.items.length > 0);
}
const visibleSwaggerUnregisteredGroups = computed(() => filterSwaggerGroups(swaggerUnregisteredGroups.value, swaggerDiffSearch.value));
const visibleSwaggerRegisteredInvalidGroups = computed(() => filterSwaggerGroups(swaggerRegisteredInvalidGroups.value, swaggerDiffSearch.value));
const swaggerSelectedCount = computed(() => {
  const values = Object.values(swaggerSelectedKeys.value);
  let selected = 0;
  for (const value of values) {
    if (value) {
      selected++;
    }
  }
  return selected;
});
const grpcUnregisteredGroups = computed(() => groupGrpcEndpoints(grpcUnregistered.value));
const grpcRegisteredInvalidGroups = computed(() => groupGrpcEndpoints(grpcRegisteredInvalid.value));
function filterGrpcGroups(groups: GrpcEndpointGroup[], query: string): GrpcEndpointGroup[] {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return groups;
  }
  return groups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => `${item.service} ${item.method} ${item.path}`.toLowerCase().includes(normalizedQuery))
    }))
    .filter((group) => group.items.length > 0);
}
const visibleGrpcUnregisteredGroups = computed(() => filterGrpcGroups(grpcUnregisteredGroups.value, grpcDiffSearch.value));
const visibleGrpcRegisteredInvalidGroups = computed(() => filterGrpcGroups(grpcRegisteredInvalidGroups.value, grpcDiffSearch.value));
const grpcSelectedCount = computed(() => {
  const values = Object.values(grpcSelectedKeys.value);
  let selected = 0;
  for (const value of values) {
    if (value) {
      selected++;
    }
  }
  return selected;
});
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
    return "Incorrectly registered";
  }
  return "Missing in Swagger";
}

function swaggerEndpointKey(item: AppSwaggerEndpoint): string {
  const method = (item.method || "").trim().toUpperCase();
  const path = normalizedRoutePath(item.path);
  return `${method} ${path}`;
}

function isSwaggerSelected(item: AppSwaggerEndpoint): boolean {
  return Boolean(swaggerSelectedKeys.value[swaggerEndpointKey(item)]);
}

function toggleSwaggerSelection(item: AppSwaggerEndpoint, nextValue: boolean) {
  const key = swaggerEndpointKey(item);
  swaggerSelectedKeys.value = {
    ...swaggerSelectedKeys.value,
    [key]: nextValue
  };
}

function clearSwaggerSelection() {
  swaggerSelectedKeys.value = {};
}

function pruneSwaggerSelection() {
  if (swaggerUnregistered.value.length === 0) {
    clearSwaggerSelection();
    return;
  }
  const allowed = new Set(swaggerUnregistered.value.map((item) => swaggerEndpointKey(item)));
  const next: Record<string, boolean> = {};
  for (const [key, selected] of Object.entries(swaggerSelectedKeys.value)) {
    if (selected && allowed.has(key)) {
      next[key] = true;
    }
  }
  swaggerSelectedKeys.value = next;
}

function onSwaggerItemSelectChange(item: AppSwaggerEndpoint, checked: boolean) {
  toggleSwaggerSelection(item, checked);
}

function buildDefaultEndpointPayload(item: AppSwaggerEndpoint): EndpointMain {
  return {
    id: "",
    app_id: id.value,
    active: true,
    method: (item.method || "").trim().toUpperCase() || "GET",
    path: normalizedRoutePath(item.path),
    backend: {
      custom_path: "",
      headers: {},
      query_params: {}
    },
    auth: {
      enabled: true,
      mode: "extend",
      methods: []
    },
    type: "http",
    grpc: {
      service: "",
      method: "",
      path: ""
    },
    variables: []
  };
}

function normalizeGrpcPath(service: string, method: string, path: string): string {
  const explicitPath = (path || "").trim();
  if (explicitPath) {
    return explicitPath.startsWith("/") ? explicitPath : `/${explicitPath}`;
  }
  const cleanService = (service || "").trim();
  const cleanMethod = (method || "").trim();
  if (!cleanService || !cleanMethod) {
    return "";
  }
  return `/${cleanService}/${cleanMethod}`;
}

function normalizeGrpcEndpoint(item: AppGrpcReflectionEndpoint): AppGrpcReflectionEndpoint {
  const service = (item.service || "").trim();
  const method = (item.method || "").trim();
  return {
    service,
    method,
    path: normalizeGrpcPath(service, method, item.path || "")
  };
}

function grpcEndpointFromRegistered(item: EndpointMain): AppGrpcReflectionEndpoint | null {
  if (endpointType(item) !== "grpc") {
    return null;
  }
  let service = (item.grpc?.service || "").trim();
  let method = (item.grpc?.method || "").trim();
  if (!service || !method) {
    const parts = normalizedRoutePath(item.path).split("/").filter(Boolean);
    if (parts.length === 2) {
      service = service || parts[0];
      method = method || parts[1];
    }
  }
  if (!service || !method) {
    return null;
  }
  const path = normalizeGrpcPath(service, method, item.grpc?.path || item.path || "");
  return {
    service,
    method,
    path
  };
}

function sortGrpcEndpoints(a: AppGrpcReflectionEndpoint, b: AppGrpcReflectionEndpoint): number {
  const serviceCompare = (a.service || "").localeCompare(b.service || "");
  if (serviceCompare !== 0) {
    return serviceCompare;
  }
  const methodCompare = (a.method || "").localeCompare(b.method || "");
  if (methodCompare !== 0) {
    return methodCompare;
  }
  return normalizeGrpcPath(a.service, a.method, a.path).localeCompare(normalizeGrpcPath(b.service, b.method, b.path));
}

function grpcDiffKey(item: AppGrpcReflectionEndpoint): string {
  return `${(item.service || "").trim()}/${(item.method || "").trim()}`;
}

function buildGrpcDiff() {
  const reflectionMap = new Map<string, AppGrpcReflectionEndpoint>();
  for (const raw of grpcReflectionEndpoints.value) {
    const item = normalizeGrpcEndpoint(raw);
    const key = grpcDiffKey(item);
    if (!item.service || !item.method || reflectionMap.has(key)) {
      continue;
    }
    reflectionMap.set(key, item);
  }

  const registeredMap = new Map<string, AppGrpcReflectionEndpoint>();
  for (const endpoint of grpcEndpoints.value) {
    const converted = grpcEndpointFromRegistered(endpoint);
    if (!converted) {
      continue;
    }
    const item = normalizeGrpcEndpoint(converted);
    const key = grpcDiffKey(item);
    if (!item.service || !item.method || registeredMap.has(key)) {
      continue;
    }
    registeredMap.set(key, item);
  }

  const unregistered: AppGrpcReflectionEndpoint[] = [];
  for (const [key, item] of reflectionMap.entries()) {
    if (!registeredMap.has(key)) {
      unregistered.push(item);
    }
  }

  const registeredInvalid: AppGrpcReflectionEndpoint[] = [];
  for (const [key, item] of registeredMap.entries()) {
    if (!reflectionMap.has(key)) {
      registeredInvalid.push(item);
    }
  }

  grpcUnregistered.value = unregistered.sort((a, b) => sortGrpcEndpoints(a, b));
  grpcRegisteredInvalid.value = registeredInvalid.sort((a, b) => sortGrpcEndpoints(a, b));
}

function grpcServiceSegment(service: string): string {
  const normalized = (service || "").trim();
  if (!normalized) {
    return "/";
  }
  const idx = normalized.lastIndexOf(".");
  if (idx < 0) {
    return normalized;
  }
  return normalized.slice(idx + 1);
}

function groupGrpcEndpoints(items: AppGrpcReflectionEndpoint[]): GrpcEndpointGroup[] {
  const groups = new Map<string, AppGrpcReflectionEndpoint[]>();
  for (const item of items) {
    const key = (item.service || "").trim() || "/";
    const current = groups.get(key);
    if (current) {
      current.push(item);
    } else {
      groups.set(key, [item]);
    }
  }

  return Array.from(groups.entries())
    .map(([service, groupItems]) => ({
      segment: grpcServiceSegment(service),
      items: [...groupItems].sort((a, b) => sortGrpcEndpoints(a, b))
    }))
    .sort((a, b) => a.segment.localeCompare(b.segment));
}

function grpcEndpointSelectionKey(item: AppGrpcReflectionEndpoint): string {
  return `${grpcDiffKey(item)} ${normalizeGrpcPath(item.service, item.method, item.path)}`;
}

function isGrpcSelected(item: AppGrpcReflectionEndpoint): boolean {
  return Boolean(grpcSelectedKeys.value[grpcEndpointSelectionKey(item)]);
}

function toggleGrpcSelection(item: AppGrpcReflectionEndpoint, nextValue: boolean) {
  const key = grpcEndpointSelectionKey(item);
  grpcSelectedKeys.value = {
    ...grpcSelectedKeys.value,
    [key]: nextValue
  };
}

function clearGrpcSelection() {
  grpcSelectedKeys.value = {};
}

function pruneGrpcSelection() {
  if (grpcUnregistered.value.length === 0) {
    clearGrpcSelection();
    return;
  }
  const allowed = new Set(grpcUnregistered.value.map((item) => grpcEndpointSelectionKey(item)));
  const next: Record<string, boolean> = {};
  for (const [key, selected] of Object.entries(grpcSelectedKeys.value)) {
    if (selected && allowed.has(key)) {
      next[key] = true;
    }
  }
  grpcSelectedKeys.value = next;
}

function onGrpcItemSelectChange(item: AppGrpcReflectionEndpoint, checked: boolean) {
  toggleGrpcSelection(item, checked);
}

function buildDefaultGrpcEndpointPayload(item: AppGrpcReflectionEndpoint): EndpointMain {
  const normalized = normalizeGrpcEndpoint(item);
  return {
    id: "",
    app_id: id.value,
    active: true,
    method: "GRPC",
    path: normalized.path,
    backend: {
      custom_path: "",
      headers: {},
      query_params: {}
    },
    auth: {
      enabled: true,
      mode: "extend",
      methods: []
    },
    type: "grpc",
    grpc: {
      service: normalized.service,
      method: normalized.method,
      path: normalized.path
    },
    variables: []
  };
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
  endpoint_type?: EndpointType;
};

type SavedSwaggerPanelState = {
  open?: boolean;
};

function endpointFiltersStorageKey(): string {
  return `app-details:endpoint-filters:${id.value || "_"}`;
}

function swaggerPanelStorageKey(): string {
  return `app-details:swagger-panel:${id.value || "_"}`;
}

function grpcPanelStorageKey(): string {
  return `app-details:grpc-panel:${id.value || "_"}`;
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
    activeEndpointType.value = parsed.endpoint_type === "grpc" ? "grpc" : "http";
  } catch {
    endpointSearch.value = "";
    authVisibilityFilter.value = "all";
    activeFilter.value = "all";
    httpMethodFilter.value = "all";
    activeEndpointType.value = "http";
  }
}

function restoreSwaggerPanelState() {
  const raw = window.sessionStorage.getItem(swaggerPanelStorageKey());
  if (!raw) {
    swaggerPanelOpen.value = false;
    return;
  }

  try {
    const parsed = JSON.parse(raw) as SavedSwaggerPanelState;
    swaggerPanelOpen.value = Boolean(parsed.open);
  } catch {
    swaggerPanelOpen.value = false;
  }
}

function restoreGrpcPanelState() {
  const raw = window.sessionStorage.getItem(grpcPanelStorageKey());
  if (!raw) {
    grpcPanelOpen.value = false;
    return;
  }

  try {
    const parsed = JSON.parse(raw) as SavedSwaggerPanelState;
    grpcPanelOpen.value = Boolean(parsed.open);
  } catch {
    grpcPanelOpen.value = false;
  }
}

function persistEndpointFilters() {
  const payload: SavedEndpointFilters = {
    endpoint_search: endpointSearch.value,
    auth_visibility_filter: authVisibilityFilter.value,
    active_filter: activeFilter.value,
    http_method_filter: httpMethodFilter.value,
    endpoint_type: activeEndpointType.value
  };
  window.sessionStorage.setItem(endpointFiltersStorageKey(), JSON.stringify(payload));
}

function persistSwaggerPanelState() {
  const payload: SavedSwaggerPanelState = {
    open: swaggerPanelOpen.value
  };
  window.sessionStorage.setItem(swaggerPanelStorageKey(), JSON.stringify(payload));
}

function persistGrpcPanelState() {
  const payload: SavedSwaggerPanelState = {
    open: grpcPanelOpen.value
  };
  window.sessionStorage.setItem(grpcPanelStorageKey(), JSON.stringify(payload));
}

function clearPersistedEndpointFilters() {
  window.sessionStorage.removeItem(endpointFiltersStorageKey());
}

function clearPersistedSwaggerPanelState() {
  window.sessionStorage.removeItem(swaggerPanelStorageKey());
}

function clearPersistedGrpcPanelState() {
  window.sessionStorage.removeItem(grpcPanelStorageKey());
}

function clearPersistedDiffPanelState() {
  clearPersistedSwaggerPanelState();
  clearPersistedGrpcPanelState();
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
  const pathCompare = endpointRoutePath(a).localeCompare(endpointRoutePath(b));
  if (pathCompare !== 0) {
    return pathCompare;
  }
  return endpointRouteMethod(a).localeCompare(endpointRouteMethod(b));
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
    icons.push({ key: "ip_validation", component: GlobeOutline, label: "IP Validation" });
  }
  if (hasJwt) {
    icons.push({ key: "jwt", component: KeyOutline, label: "JWT" });
  }
  if (hasBasic) {
    icons.push({ key: "basic", component: PersonOutline, label: "Basic Auth" });
  }
  if (hasApiKey) {
    icons.push({ key: "api_key", component: KeyOutline, label: "API Key" });
  }
  return icons;
}

function endpointRequiresAuth(endpoint: EndpointMain): boolean {
  return Boolean(endpoint.auth?.enabled);
}

function endpointMethodBadgeClass(method: string): string {
  const normalized = (method || "").trim().toUpperCase();
  switch (normalized) {
    case "GRPC":
      return "method-grpc";
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

function endpointType(endpoint: EndpointMain): EndpointType {
  return endpoint.type === "grpc" ? "grpc" : "http";
}

function endpointGroupKey(endpoint: EndpointMain): string {
  if (endpointType(endpoint) === "grpc") {
    const service = (endpoint.grpc?.service || "").trim();
    if (service) {
      return service;
    }
  }
  return firstPathSegment(endpointRoutePath(endpoint));
}

function endpointGroupSegment(endpoint: EndpointMain, key: string): string {
  if (endpointType(endpoint) === "grpc") {
    const normalized = key.trim().replace(/^\//, "");
    const methodIndex = normalized.indexOf("/");
    const service = methodIndex >= 0 ? normalized.slice(0, methodIndex) : normalized;
    if (!service) {
      return "/";
    }
    const shortIndex = service.lastIndexOf(".");
    return shortIndex >= 0 ? service.slice(shortIndex + 1) : service;
  }
  return key;
}

function endpointRoutePath(endpoint: EndpointMain): string {
  if (endpointType(endpoint) === "grpc") {
    return normalizedRoutePath(endpoint.grpc?.path || endpoint.path);
  }
  return normalizedRoutePath(endpoint.path);
}

function endpointDisplayPath(endpoint: EndpointMain): string {
  if (endpointType(endpoint) === "grpc") {
    const method = (endpoint.grpc?.method || "").trim();
    if (method) {
      return `/${method}`;
    }
    const normalized = endpointRoutePath(endpoint);
    const parts = normalized.split("/").filter(Boolean);
    if (parts.length === 0) {
      return normalized;
    }
    return `/${parts[parts.length - 1]}`;
  }
  return endpointRoutePath(endpoint);
}

function endpointRouteMethod(endpoint: EndpointMain): string {
  if (endpointType(endpoint) === "grpc") {
    return "GRPC";
  }
  return (endpoint.method || "").trim().toUpperCase() || "*";
}

function endpointRouteTitle(endpoint: EndpointMain): string {
  if (endpointType(endpoint) === "grpc") {
    return `${endpoint.grpc?.service || ""}/${endpoint.grpc?.method || ""}`.trim() || endpointRoutePath(endpoint);
  }
  return `${endpointRouteMethod(endpoint)} ${endpointRoutePath(endpoint)}`;
}

function resetEndpointFilters() {
  endpointSearch.value = "";
  authVisibilityFilter.value = "all";
  activeFilter.value = "all";
  httpMethodFilter.value = "all";
}

watch(protocolOptions, (options) => {
  if (!options.find((o) => o.value === activeEndpointType.value)) {
    activeEndpointType.value = "http";
  }
});

async function load() {
  loading.value = true;
  errorMessage.value = "";
  swaggerDiffError.value = "";
  swaggerUnregistered.value = [];
  swaggerRegisteredInvalid.value = [];
  swaggerDiffLoaded.value = false;
  grpcReflectionEndpoints.value = [];
  grpcUnregistered.value = [];
  grpcRegisteredInvalid.value = [];
  grpcDiffLoaded.value = false;
  try {
    app.value = await getApp(id.value);
    root.value = await getRoot().catch(() => null);
    const endpointList = await listEndpoints({
      app_id: id.value
    });
    endpoints.value = endpointList.results;
    if (!app.value.backend.swagger_url) {
      swaggerPanelOpen.value = false;
    }
    if (Number(app.value.backend.grpc_port || 0) <= 0) {
      grpcPanelOpen.value = false;
    }
    if (swaggerPanelOpen.value) {
      void loadSwaggerDiff();
    }
    if (grpcPanelOpen.value) {
      void loadGrpcDiff();
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load application";
  } finally {
    loading.value = false;
    swaggerDiffLoading.value = false;
    grpcDiffLoading.value = false;
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
    pruneSwaggerSelection();
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
    grpcPanelOpen.value = false;
    grpcInstructionOpen.value = false;
    void loadSwaggerDiff();
  }
}

function closeSwaggerPanel() {
  swaggerPanelOpen.value = false;
}

async function loadGrpcDiff() {
  if (!app.value || Number(app.value.backend.grpc_port || 0) <= 0 || grpcDiffLoading.value || grpcDiffLoaded.value) {
    return;
  }

  grpcDiffLoading.value = true;
  grpcDiffError.value = "";
  try {
    const diffRep = await getAppGrpcReflectionEndpoints(id.value);
    grpcReflectionEndpoints.value = diffRep.results || [];
    buildGrpcDiff();
    pruneGrpcSelection();
    grpcDiffLoaded.value = true;
  } catch (error) {
    grpcDiffError.value = error instanceof Error ? error.message : "Unable to load gRPC reflection endpoints";
    grpcReflectionEndpoints.value = [];
    grpcUnregistered.value = [];
    grpcRegisteredInvalid.value = [];
    clearGrpcSelection();
  } finally {
    grpcDiffLoading.value = false;
  }
}

function toggleGrpcPanel() {
  if (!app.value || Number(app.value.backend.grpc_port || 0) <= 0) {
    return;
  }
  grpcPanelOpen.value = !grpcPanelOpen.value;
  if (grpcPanelOpen.value) {
    swaggerPanelOpen.value = false;
    grpcInstructionOpen.value = false;
    void loadGrpcDiff();
  }
}

function closeGrpcPanel() {
  grpcPanelOpen.value = false;
}

function toggleGrpcInstruction() {
  grpcInstructionOpen.value = !grpcInstructionOpen.value;
  if (grpcInstructionOpen.value) {
    grpcPanelOpen.value = false;
    swaggerPanelOpen.value = false;
  }
}

function closeDiffPanels() {
  swaggerPanelOpen.value = false;
  grpcPanelOpen.value = false;
}

function closeDiffPanelsAndClearState() {
  closeDiffPanels();
  clearPersistedDiffPanelState();
}

function onDiffModalKeydown(event: KeyboardEvent) {
  if (event.key !== "Escape") {
    return;
  }
  if (swaggerPanelOpen.value || grpcPanelOpen.value) {
    closeDiffPanels();
  }
}

function toggleSwaggerRegisteredInvalid() {
  swaggerRegisteredInvalidOpen.value = !swaggerRegisteredInvalidOpen.value;
}

function toggleGrpcRegisteredInvalid() {
  grpcRegisteredInvalidOpen.value = !grpcRegisteredInvalidOpen.value;
}

async function addSelectedSwaggerEndpoints() {
  if (swaggerBulkAdding.value || swaggerSelectedCount.value === 0) {
    return;
  }

  const selectedItems = swaggerUnregistered.value.filter((item) => isSwaggerSelected(item));
  if (selectedItems.length === 0) {
    return;
  }

  dialog.info({
    title: "Add endpoints",
    content: `Add ${selectedItems.length} endpoint(s) with default settings?`,
    positiveText: "Add",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runAddSelectedSwaggerEndpoints(selectedItems);
    }
  });
}

async function runAddSelectedSwaggerEndpoints(selectedItems: AppSwaggerEndpoint[]) {
  swaggerBulkAdding.value = true;
  errorMessage.value = "";

  let successCount = 0;
  let failureCount = 0;

  try {
    for (const item of selectedItems) {
      try {
        await createEndpoint(buildDefaultEndpointPayload(item));
        successCount++;
      } catch {
        failureCount++;
      }
    }

    if (successCount > 0) {
      notifySuccess(`Added ${successCount} endpoint(s)`);
      clearSwaggerSelection();
      swaggerDiffLoaded.value = false;
      closeSwaggerPanel();
      await load();
    }

    if (failureCount > 0) {
      notifyError(`Failed to add ${failureCount} endpoint(s). Check duplicates or validation.`);
    }
  } finally {
    swaggerBulkAdding.value = false;
  }
}

async function addSelectedGrpcEndpoints() {
  if (grpcBulkAdding.value || grpcSelectedCount.value === 0) {
    return;
  }

  const selectedItems = grpcUnregistered.value.filter((item) => isGrpcSelected(item));
  if (selectedItems.length === 0) {
    return;
  }

  dialog.info({
    title: "Add gRPC endpoints",
    content: `Add ${selectedItems.length} gRPC method(s) with default settings?`,
    positiveText: "Add",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runAddSelectedGrpcEndpoints(selectedItems);
    }
  });
}

async function runAddSelectedGrpcEndpoints(selectedItems: AppGrpcReflectionEndpoint[]) {
  grpcBulkAdding.value = true;
  errorMessage.value = "";

  let successCount = 0;
  let failureCount = 0;

  try {
    for (const item of selectedItems) {
      try {
        await createEndpoint(buildDefaultGrpcEndpointPayload(item));
        successCount++;
      } catch {
        failureCount++;
      }
    }

    if (successCount > 0) {
      notifySuccess(`Added ${successCount} gRPC endpoint(s)`);
      clearGrpcSelection();
      grpcDiffLoaded.value = false;
      closeGrpcPanel();
      await load();
    }

    if (failureCount > 0) {
      notifyError(`Failed to add ${failureCount} gRPC endpoint(s). Check duplicates or validation.`);
    }
  } finally {
    grpcBulkAdding.value = false;
  }
}

async function removeEndpoint(endpoint: EndpointMain) {
  if (deletingEndpointId.value) {
    return;
  }
  dialog.error({
    title: "Delete endpoint",
    content: `Delete endpoint ${endpointRouteTitle(endpoint)}?`,
    positiveText: "Delete",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runRemoveEndpoint(endpoint);
    }
  });
}

async function runRemoveEndpoint(endpoint: EndpointMain) {
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
  if (!app.value) {
    return;
  }
  dialog.error({
    title: "Delete application",
    content: `Delete application "${app.value.name || app.value.id}"?`,
    positiveText: "Delete",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runRemoveApp();
    }
  });
}

async function runRemoveApp() {
  if (!app.value) {
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
  dialog.warning({
    title: `${action} application`,
    content: `${action} application "${app.value.name || app.value.id}"?`,
    positiveText: action,
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runToggleAppActive(nextActive);
    }
  });
}

async function runToggleAppActive(nextActive: boolean) {
  if (!app.value) {
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
  restoreSwaggerPanelState();
  restoreGrpcPanelState();
  window.addEventListener("keydown", onDiffModalKeydown);
  void load();
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onDiffModalKeydown);
});

watch([id, endpointSearch, authVisibilityFilter, activeFilter, httpMethodFilter, activeEndpointType], () => {
  persistEndpointFilters();
});

watch([id, swaggerPanelOpen], () => {
  persistSwaggerPanelState();
});
watch([id, grpcPanelOpen], () => {
  persistGrpcPanelState();
});

watch(swaggerUnregistered, () => {
  pruneSwaggerSelection();
});
watch(grpcUnregistered, () => {
  pruneGrpcSelection();
});

onBeforeRouteLeave((to) => {
  closeDiffPanelsAndClearState();

  if (!isWithinCurrentAppContext(to)) {
    clearPersistedEndpointFilters();
  }
});
</script>

<template>
  <div class="actions page-top-actions app-details-top-actions">
    <div v-if="app" class="app-details-header-meta">
      <div class="app-details-page-title">{{ app.name }}</div>
      <n-tag class="app-details-status-badge" size="small" :type="app.active ? 'success' : 'warning'">
        {{ app.active ? "active" : "inactive" }}
      </n-tag>
    </div>
    <n-button
      v-if="app"
      :type="app.active ? 'error' : 'primary'"
      :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
      :loading="togglingApp"
      :title="app.active ? 'Deactivate App' : 'Activate App'"
      @click="toggleAppActive"
    >
      {{ togglingApp ? "Saving..." : app.active ? "Deactivate App" : "Activate App" }}
    </n-button>
    <RouterLink
      class="icon-action-button secondary"
      :to="{ name: 'app-edit', params: { id } }"
      title="Edit App"
      aria-label="Edit App"
    >
      <n-icon :component="CreateOutline" />
    </RouterLink>
    <n-button
      class="danger-icon-button"
      type="error"
      secondary
      circle
      :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
      :loading="deletingApp"
      title="Delete App"
      aria-label="Delete App"
      @click="removeApp"
    >
      <n-icon :component="TrashOutline" />
    </n-button>
  </div>

  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
  <p v-if="loading" class="muted">Loading...</p>

  <template v-else-if="app">
    <section class="summary-grid">
      <div>
        <span class="label">Path Prefix</span>
        <strong>{{ app.path_prefix }}</strong>
      </div>
      <div>
        <span class="label">Backend</span>
        <strong>{{ app.backend.url }}</strong>
      </div>
      <div>
        <span class="label">gRPC Port</span>
        <strong>{{ app.backend.grpc_port || "disabled" }}</strong>
      </div>
      <div>
        <span class="label">Status</span>
        <strong>{{ app.active ? "active" : "inactive" }}</strong>
      </div>
    </section>

    <div v-if="protocolOptions.length > 1" class="app-protocol-card">
      <div class="protocol-tabs-wrap">
        <n-tabs v-model:value="activeEndpointType" class="protocol-tabs" type="line" size="small" animated>
          <n-tab-pane
            v-for="option in protocolOptions"
            :key="option.value"
            :name="option.value"
            display-directive="show"
          >
            <template #tab>
              <span>{{ option.label }}</span>
              <span class="protocol-tab-count">{{ option.value === "grpc" ? grpcEndpoints.length : httpEndpoints.length }}</span>
            </template>
          </n-tab-pane>
        </n-tabs>
      </div>
    </div>

    <GrpcInstructionPanel
      v-if="activeEndpointType === 'grpc' && app"
      :app="app"
      :root="root"
      :open="grpcInstructionOpen"
      @close="grpcInstructionOpen = false"
    />

    <n-modal
      :show="Boolean(activeEndpointType === 'http' && app.backend.swagger_url && swaggerPanelOpen)"
      preset="card"
      title="Swagger Diff"
      class="diff-modal-card"
      :bordered="false"
      :mask-closable="true"
      content-style="display: flex; min-height: 0; height: 100%; overflow: hidden;"
      @close="closeSwaggerPanel"
      @update:show="(value: boolean) => { if (!value) closeSwaggerPanel(); }"
    >
          <div class="swagger-diff-panel diff-modal">
            <p v-if="swaggerDiffLoading" class="muted">Loading swagger endpoints...</p>
            <n-alert v-else-if="swaggerDiffError" class="form-alert" type="error" :show-icon="false">{{ swaggerDiffError }}</n-alert>
            <template v-else>
              <div class="diff-toolbar">
                <n-input v-model:value="swaggerDiffSearch" class="diff-search-input" placeholder="Search diff" clearable>
                  <template #prefix>
                    <n-icon :component="SearchOutline" />
                  </template>
                </n-input>
                <n-button v-if="swaggerRegisteredInvalid.length > 0" class="swagger-invalid-toggle-button" secondary @click="toggleSwaggerRegisteredInvalid">
                  {{
                    swaggerRegisteredInvalidOpen
                      ? `Hide Registered Invalid (${swaggerRegisteredInvalid.length})`
                      : `Show Registered Invalid (${swaggerRegisteredInvalid.length})`
                  }}
                </n-button>
                <n-button
                  v-if="swaggerUnregistered.length > 0"
                  class="swagger-bulk-add-button"
                  type="primary"
                  :disabled="swaggerBulkAdding || swaggerSelectedCount === 0"
                  :loading="swaggerBulkAdding"
                  @click="addSelectedSwaggerEndpoints"
                >
                  {{ swaggerBulkAdding ? "Adding..." : `Add selected (${swaggerSelectedCount})` }}
                </n-button>
              </div>
              <div class="swagger-diff-grid diff-scroll-area">
                <div v-if="swaggerRegisteredInvalidOpen" class="swagger-diff-column">
                  <div class="swagger-diff-title-row">
                    <div class="label">Registered Invalid</div>
                    <span class="swagger-diff-count">{{ swaggerRegisteredInvalid.length }}</span>
                  </div>
                  <div class="swagger-section-box">
                    <div v-if="visibleSwaggerRegisteredInvalidGroups.length > 0" class="swagger-endpoint-groups">
                      <section
                        v-for="group in visibleSwaggerRegisteredInvalidGroups"
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
                <div class="swagger-diff-column">
                  <div class="swagger-diff-title-row">
                    <div class="label">Not Registered</div>
                    <span class="swagger-diff-count">{{ swaggerUnregistered.length }}</span>
                  </div>
                  <div class="swagger-section-box">
                    <div v-if="visibleSwaggerUnregisteredGroups.length > 0" class="swagger-endpoint-groups">
                      <section
                        v-for="group in visibleSwaggerUnregisteredGroups"
                        :key="`missing-group-${group.segment}`"
                        class="swagger-endpoint-group"
                      >
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
                            <n-checkbox
                              class="swagger-item-select"
                              :checked="isSwaggerSelected(item)"
                              :disabled="swaggerBulkAdding"
                              @update:checked="(checked: boolean) => onSwaggerItemSelectChange(item, checked)"
                            />
                            <span class="http-method-badge" :class="endpointMethodBadgeClass(item.method)">{{ item.method }}</span>
                            <span class="swagger-endpoint-path">{{ item.path }}</span>
                            <RouterLink
                              class="primary-button swagger-add-button"
                              :to="{
                                name: 'endpoint-create',
                                params: { appId: id },
                                query: { type: 'http', method: item.method, path: item.path }
                              }"
                              :aria-label="`Add endpoint ${item.method} ${item.path}`"
                              @click="closeDiffPanelsAndClearState"
                            >
                              <n-icon class="swagger-add-plus" :component="AddOutline" />
                              <span>Add</span>
                            </RouterLink>
                          </li>
                        </ul>
                      </section>
                    </div>
                    <p v-else class="muted">No missing endpoints.</p>
                  </div>
                </div>
              </div>
            </template>
          </div>
    </n-modal>
    <n-modal
      :show="Boolean(activeEndpointType === 'grpc' && Number(app.backend.grpc_port || 0) > 0 && grpcPanelOpen)"
      preset="card"
      title="gRPC Reflection Diff"
      class="diff-modal-card"
      :bordered="false"
      :mask-closable="true"
      content-style="display: flex; min-height: 0; height: 100%; overflow: hidden;"
      @close="closeGrpcPanel"
      @update:show="(value: boolean) => { if (!value) closeGrpcPanel(); }"
    >
          <div class="swagger-diff-panel grpc-diff-panel diff-modal">
            <p v-if="grpcDiffLoading" class="muted">Loading gRPC reflection methods...</p>
            <n-alert v-else-if="grpcDiffError" class="form-alert" type="error" :show-icon="false">{{ grpcDiffError }}</n-alert>
            <template v-else>
              <div class="diff-toolbar">
                <n-input v-model:value="grpcDiffSearch" class="diff-search-input" placeholder="Search diff" clearable>
                  <template #prefix>
                    <n-icon :component="SearchOutline" />
                  </template>
                </n-input>
                <n-button v-if="grpcRegisteredInvalid.length > 0" class="swagger-invalid-toggle-button" secondary @click="toggleGrpcRegisteredInvalid">
                  {{
                    grpcRegisteredInvalidOpen
                      ? `Hide Registered Invalid (${grpcRegisteredInvalid.length})`
                      : `Show Registered Invalid (${grpcRegisteredInvalid.length})`
                  }}
                </n-button>
                <n-button
                  v-if="grpcUnregistered.length > 0"
                  class="swagger-bulk-add-button"
                  type="primary"
                  :disabled="grpcBulkAdding || grpcSelectedCount === 0"
                  :loading="grpcBulkAdding"
                  @click="addSelectedGrpcEndpoints"
                >
                  {{ grpcBulkAdding ? "Adding..." : `Add selected (${grpcSelectedCount})` }}
                </n-button>
              </div>
              <div class="swagger-diff-grid diff-scroll-area">
                <div v-if="grpcRegisteredInvalidOpen" class="swagger-diff-column">
                  <div class="swagger-diff-title-row">
                    <div class="label">Registered Invalid</div>
                    <span class="swagger-diff-count">{{ grpcRegisteredInvalid.length }}</span>
                  </div>
                  <div class="swagger-section-box">
                    <div v-if="visibleGrpcRegisteredInvalidGroups.length > 0" class="swagger-endpoint-groups">
                      <section
                        v-for="group in visibleGrpcRegisteredInvalidGroups"
                        :key="`grpc-invalid-group-${group.segment}`"
                        class="swagger-endpoint-group"
                      >
                        <div class="endpoint-group-head">
                          <span class="endpoint-group-segment">{{ group.segment }}</span>
                          <span class="endpoint-group-count">{{ group.items.length }}</span>
                        </div>
                        <ul class="swagger-endpoint-list">
                          <li
                            v-for="item in group.items"
                            :key="`grpc-invalid-${group.segment}-${item.service}-${item.method}`"
                            class="swagger-endpoint-item swagger-endpoint-item-invalid"
                          >
                            <span class="http-method-badge method-grpc">GRPC</span>
                            <div class="grpc-endpoint-meta">
                              <span class="swagger-endpoint-path" :title="`${item.service}/${item.method}`">/{{ item.method }}</span>
                              <span class="swagger-endpoint-reason">Missing in reflection</span>
                            </div>
                          </li>
                        </ul>
                      </section>
                    </div>
                    <p v-else class="muted">No invalid registrations.</p>
                  </div>
                </div>
                <div class="swagger-diff-column">
                  <div class="swagger-diff-title-row">
                    <div class="label">Not Registered</div>
                    <span class="swagger-diff-count">{{ grpcUnregistered.length }}</span>
                  </div>
                  <div class="swagger-section-box">
                    <div v-if="visibleGrpcUnregisteredGroups.length > 0" class="swagger-endpoint-groups">
                      <section
                        v-for="group in visibleGrpcUnregisteredGroups"
                        :key="`grpc-missing-group-${group.segment}`"
                        class="swagger-endpoint-group"
                      >
                        <div class="endpoint-group-head">
                          <span class="endpoint-group-segment">{{ group.segment }}</span>
                          <span class="endpoint-group-count">{{ group.items.length }}</span>
                        </div>
                        <ul class="swagger-endpoint-list">
                          <li
                            v-for="item in group.items"
                            :key="`grpc-missing-${group.segment}-${item.service}-${item.method}`"
                            class="swagger-endpoint-item"
                          >
                            <n-checkbox
                              class="swagger-item-select"
                              :checked="isGrpcSelected(item)"
                              :disabled="grpcBulkAdding"
                              @update:checked="(checked: boolean) => onGrpcItemSelectChange(item, checked)"
                            />
                            <span class="http-method-badge method-grpc">GRPC</span>
                            <span class="swagger-endpoint-path" :title="`${item.service}/${item.method}`">/{{ item.method }}</span>
                            <RouterLink
                              class="primary-button swagger-add-button"
                              :to="{
                                name: 'endpoint-create',
                                params: { appId: id },
                                query: { type: 'grpc', grpc_service: item.service, grpc_method: item.method, grpc_path: item.path }
                              }"
                              :aria-label="`Add endpoint ${item.service}/${item.method}`"
                              @click="closeDiffPanelsAndClearState"
                            >
                              <n-icon class="swagger-add-plus" :component="AddOutline" />
                              <span>Add</span>
                            </RouterLink>
                          </li>
                        </ul>
                      </section>
                    </div>
                    <p v-else class="muted">No missing methods.</p>
                  </div>
                </div>
              </div>
            </template>
          </div>
    </n-modal>

    <div class="page-header compact endpoints-head">
      <div class="endpoints-title">
        <h3>{{ activeEndpointType === "grpc" ? "gRPC Endpoints" : "HTTP Endpoints" }}</h3>
        <span class="muted endpoints-head-count">{{ filteredEndpoints.length }} / {{ activeProtocolTotal }}</span>
      </div>
      <div class="actions endpoints-head-actions">
        <n-button
          v-if="activeEndpointType === 'grpc'"
          class="grpc-connect-button"
          type="info"
          secondary
          :disabled="loading || deletingApp || deletingEndpointId !== '' || togglingApp"
          title="Open gRPC connection guide"
          aria-label="Open gRPC connection guide"
          @click="toggleGrpcInstruction"
        >
          <template #icon>
            <n-icon :component="TerminalOutline" />
          </template>
          Connect
        </n-button>
        <n-button
          v-if="activeEndpointType === 'http' || Number(app?.backend.grpc_port || 0) > 0"
          type="primary"
          :disabled="loading || deletingApp || deletingEndpointId !== '' || togglingApp"
          @click="router.push({ name: 'endpoint-create', params: { appId: id }, query: { type: activeEndpointType } })"
        >
          <template #icon>
            <n-icon :component="AddOutline" />
          </template>
          Endpoint
        </n-button>
        <n-button
          v-if="activeEndpointType === 'http' && app?.backend.swagger_url"
          class="swagger-toggle-button"
          :disabled="loading || deletingApp || deletingEndpointId !== '' || togglingApp"
          secondary
          @click="toggleSwaggerPanel"
        >
          <template #icon>
            <n-icon :component="DocumentTextOutline" />
          </template>
          Swagger
        </n-button>
        <n-button
          v-if="activeEndpointType === 'grpc' && Number(app?.backend.grpc_port || 0) > 0"
          class="grpc-reflection-toggle-button"
          :disabled="loading || deletingApp || deletingEndpointId !== '' || togglingApp"
          secondary
          @click="toggleGrpcPanel"
        >
          <template #icon>
            <n-icon :component="GitCompareOutline" />
          </template>
          gRPC Reflection
        </n-button>
      </div>
    </div>
    <div class="endpoints-filters">
      <n-input
        v-model:value="endpointSearch"
        class="endpoints-filter-input"
        placeholder="Search endpoints"
        aria-label="Search endpoints"
        clearable
      >
        <template #prefix>
          <n-icon :component="SearchOutline" />
        </template>
      </n-input>
      <n-select v-model:value="authVisibilityFilter" class="endpoints-filter-select" :options="authVisibilityOptions" aria-label="Filter by visibility" />
      <n-select v-model:value="activeFilter" class="endpoints-filter-select" :options="activeFilterOptions" aria-label="Filter by status" />
      <n-select
        v-if="activeEndpointType === 'http'"
        v-model:value="httpMethodFilter"
        class="endpoints-filter-select"
        :options="httpMethodFilterOptions"
        aria-label="Filter by HTTP method"
      />
      <n-button
        secondary
        :disabled="!hasEndpointFilters"
        title="Reset Filters"
        aria-label="Reset Filters"
        @click="resetEndpointFilters"
      >
        <n-icon :component="RefreshOutline" />
      </n-button>
    </div>
    <table class="data-table endpoints-table">
      <tbody v-if="endpointGroups.length === 0">
        <tr>
          <td colspan="5" class="muted">{{ hasEndpointFilters ? "No endpoints match filters." : "No endpoints found." }}</td>
        </tr>
      </tbody>

      <tbody v-for="(group, groupIndex) in endpointGroups" :key="group.key">
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
            <span class="http-method-badge" :class="endpointMethodBadgeClass(endpointRouteMethod(endpoint))">{{ endpointRouteMethod(endpoint) }}</span>
          </td>
          <td class="endpoint-path-cell">
            <RouterLink
              class="endpoint-path-link"
              :to="{ name: 'endpoint-details', params: { id: endpoint.id } }"
              :title="endpointRouteTitle(endpoint)"
            >
              {{ endpointDisplayPath(endpoint) }}
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
                <n-icon :component="LockClosedOutline" />
              </span>
              <div v-if="endpointAuthIcons(endpoint).length > 0" class="endpoint-auth-icons">
                <span
                  v-for="authIcon in endpointAuthIcons(endpoint)"
                  :key="authIcon.key"
                  class="endpoint-auth-icon"
                  :title="authIcon.label"
                  :aria-label="authIcon.label"
                >
                  <n-icon :component="authIcon.component" />
                </span>
              </div>
            </div>
          </td>
          <td>
            <n-tag size="small" :type="endpoint.active ? 'success' : 'warning'">
              {{ endpoint.active ? "active" : "inactive" }}
            </n-tag>
          </td>
          <td class="actions">
            <RouterLink
              class="icon-action-button secondary"
              :to="{ name: 'endpoint-edit', params: { id: endpoint.id } }"
              title="Edit Endpoint"
              aria-label="Edit Endpoint"
            >
              <n-icon :component="CreateOutline" />
            </RouterLink>
            <n-button
              class="danger-icon-button"
              type="error"
              secondary
              size="small"
              circle
              :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
              title="Delete Endpoint"
              aria-label="Delete Endpoint"
              @click="removeEndpoint(endpoint)"
            >
              <n-icon v-if="deletingEndpointId !== endpoint.id" :component="TrashOutline" />
            </n-button>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="endpoints-mobile-list">
      <p v-if="endpointGroups.length === 0" class="muted endpoints-mobile-empty">
        {{ hasEndpointFilters ? "No endpoints match filters." : "No endpoints found." }}
      </p>
      <section v-for="group in endpointGroups" :key="`mobile-${group.key}`" class="endpoint-mobile-group">
        <div class="endpoint-group-head">
          <span class="endpoint-group-segment">{{ group.segment }}</span>
          <span class="endpoint-group-count">{{ group.items.length }}</span>
        </div>
        <div class="endpoint-mobile-cards">
          <article v-for="endpoint in group.items" :key="`mobile-${endpoint.id}`" class="endpoint-mobile-card">
            <div class="endpoint-mobile-top">
              <span class="http-method-badge" :class="endpointMethodBadgeClass(endpointRouteMethod(endpoint))">{{ endpointRouteMethod(endpoint) }}</span>
              <n-tag size="small" :type="endpoint.active ? 'success' : 'warning'">
                {{ endpoint.active ? "active" : "inactive" }}
              </n-tag>
            </div>
            <RouterLink
              class="endpoint-path-link endpoint-mobile-path"
              :to="{ name: 'endpoint-details', params: { id: endpoint.id } }"
              :title="endpointRouteTitle(endpoint)"
            >
              {{ endpointDisplayPath(endpoint) }}
            </RouterLink>
            <div class="endpoint-auth endpoint-mobile-auth">
              <span
                v-if="endpointRequiresAuth(endpoint)"
                class="endpoint-lock-chip"
                title="Auth required"
                aria-label="Auth required"
                >
                  <n-icon :component="LockClosedOutline" />
                </span>
              <div v-if="endpointAuthIcons(endpoint).length > 0" class="endpoint-auth-icons">
                <span
                  v-for="authIcon in endpointAuthIcons(endpoint)"
                  :key="`mobile-${endpoint.id}-${authIcon.key}`"
                  class="endpoint-auth-icon"
                  :title="authIcon.label"
                  :aria-label="authIcon.label"
                >
                  <n-icon :component="authIcon.component" />
                </span>
              </div>
            </div>
            <div class="endpoint-mobile-actions">
              <RouterLink
                class="icon-action-button secondary"
                :to="{ name: 'endpoint-edit', params: { id: endpoint.id } }"
                title="Edit Endpoint"
                aria-label="Edit Endpoint"
              >
                <n-icon :component="CreateOutline" />
              </RouterLink>
              <n-button
                class="danger-icon-button"
                type="error"
                secondary
                size="small"
                circle
                :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
                title="Delete Endpoint"
                aria-label="Delete Endpoint"
                @click="removeEndpoint(endpoint)"
              >
                <n-icon v-if="deletingEndpointId !== endpoint.id" :component="TrashOutline" />
              </n-button>
            </div>
          </article>
        </div>
      </section>
    </div>
  </template>
</template>
