<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave, RouterLink, useRoute, useRouter, type RouteLocationNormalizedLoaded } from "vue-router";
import {
  createEndpoint,
  deleteApp,
  deleteEndpoint,
  getApp,
  getAppGrpcReflectionEndpoints,
  getAppSwaggerEndpointsDiff,
  listEndpoints,
  updateApp
} from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppGrpcReflectionEndpoint, AppMain, AppSwaggerEndpoint, EndpointMain, EndpointType } from "../types/api";
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

const protocolOptions: Array<{ value: EndpointType; label: string }> = [
  { value: "http", label: "HTTP" },
  { value: "grpc", label: "gRPC" }
];
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

function onSwaggerItemSelectChange(item: AppSwaggerEndpoint, event: Event) {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }
  toggleSwaggerSelection(item, target.checked);
}

function buildDefaultEndpointPayload(item: AppSwaggerEndpoint): EndpointMain {
  return {
    id: "",
    app_id: id.value,
    active: true,
    method: (item.method || "").trim().toUpperCase() || "GET",
    path: normalizedRoutePath(item.path),
    backend: {
      custom_path: ""
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
    }
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

function onGrpcItemSelectChange(item: AppGrpcReflectionEndpoint, event: Event) {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }
  toggleGrpcSelection(item, target.checked);
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
      custom_path: ""
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
    }
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
    void loadGrpcDiff();
  }
}

function closeGrpcPanel() {
  grpcPanelOpen.value = false;
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

  const approved = window.confirm(`Add ${selectedItems.length} endpoint(s) with default settings?`);
  if (!approved) {
    return;
  }

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

  const approved = window.confirm(`Add ${selectedItems.length} gRPC method(s) with default settings?`);
  if (!approved) {
    return;
  }

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
  const approved = window.confirm(`Delete endpoint ${endpointRouteTitle(endpoint)}?`);
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
  restoreSwaggerPanelState();
  restoreGrpcPanelState();
  void load();
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
  if (!isWithinCurrentAppContext(to)) {
    clearPersistedEndpointFilters();
    clearPersistedSwaggerPanelState();
    clearPersistedGrpcPanelState();
  }
});
</script>

<template>
  <div class="actions page-top-actions app-details-top-actions">
    <div v-if="app" class="app-details-header-meta">
      <div class="app-details-page-title">{{ app.name }}</div>
      <span class="status-chip app-details-status-badge" :class="{ inactive: !app.active }">
        {{ app.active ? "active" : "inactive" }}
      </span>
    </div>
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
      <span class="icon-action-glyph">🗑</span>
    </button>
  </div>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
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

    <div class="app-protocol-card">
      <div class="protocol-tabs" role="tablist" aria-label="Endpoint Protocol">
        <button
          v-for="option in protocolOptions"
          :key="option.value"
          class="protocol-tab"
          :class="{ active: activeEndpointType === option.value }"
          type="button"
          :aria-selected="activeEndpointType === option.value"
          @click="activeEndpointType = option.value"
        >
          <span>{{ option.label }}</span>
          <span class="protocol-tab-count">{{ option.value === "grpc" ? grpcEndpoints.length : httpEndpoints.length }}</span>
        </button>
      </div>
      <div class="actions protocol-actions">
        <RouterLink
          class="primary-button"
          :to="{ name: 'endpoint-create', params: { appId: id }, query: { type: activeEndpointType } }"
        >
          Create Endpoint
        </RouterLink>
        <button
          v-if="activeEndpointType === 'http' && app?.backend.swagger_url"
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
          v-if="activeEndpointType === 'grpc' && Number(app?.backend.grpc_port || 0) > 0"
          class="secondary-button grpc-reflection-toggle-button"
          :disabled="loading || deletingApp || deletingEndpointId !== '' || togglingApp"
          @click="toggleGrpcPanel"
        >
          <svg class="icon-action-svg swagger-toggle-icon" viewBox="0 0 24 24" aria-hidden="true">
            <path d="M9 4H5a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h4" />
            <path d="M15 4h4a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1h-4" />
            <path d="M10 8h4" />
            <path d="M10 12h4" />
            <path d="M10 16h4" />
          </svg>
          <span>gRPC Reflection</span>
        </button>
      </div>
    </div>

    <Transition name="swagger-panel">
      <section v-if="activeEndpointType === 'http' && app.backend.swagger_url && swaggerPanelOpen" class="panel swagger-diff-panel">
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
          <div v-if="swaggerRegisteredInvalid.length > 0" class="swagger-invalid-toggle-wrap">
            <button class="secondary-button swagger-invalid-toggle-button" type="button" @click="toggleSwaggerRegisteredInvalid">
              {{
                swaggerRegisteredInvalidOpen
                  ? `Hide Registered Invalid (${swaggerRegisteredInvalid.length})`
                  : `Show Registered Invalid (${swaggerRegisteredInvalid.length})`
              }}
            </button>
          </div>
          <div class="swagger-diff-grid">
            <div v-if="swaggerRegisteredInvalidOpen" class="swagger-diff-column">
              <div class="swagger-diff-title-row">
                <div class="label">Registered Invalid</div>
                <span class="swagger-diff-count">{{ swaggerRegisteredInvalid.length }}</span>
              </div>
              <div class="swagger-section-box">
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
            <div class="swagger-diff-column">
              <div class="swagger-diff-title-row">
                <div class="label">Not Registered</div>
                <span class="swagger-diff-count">{{ swaggerUnregistered.length }}</span>
              </div>
              <div class="swagger-section-box">
                <div v-if="swaggerUnregistered.length > 0" class="swagger-bulk-actions">
                  <button
                    class="primary-button swagger-bulk-add-button"
                    type="button"
                    :disabled="swaggerBulkAdding || swaggerSelectedCount === 0"
                    @click="addSelectedSwaggerEndpoints"
                  >
                    {{ swaggerBulkAdding ? "Adding..." : `Add selected (${swaggerSelectedCount})` }}
                  </button>
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
                        <label class="swagger-item-select">
                          <input
                            type="checkbox"
                            :checked="isSwaggerSelected(item)"
                            :disabled="swaggerBulkAdding"
                            @change="onSwaggerItemSelectChange(item, $event)"
                          />
                        </label>
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
            </div>
          </div>
        </template>
      </section>
    </Transition>
    <Transition name="swagger-panel">
      <section v-if="activeEndpointType === 'grpc' && Number(app.backend.grpc_port || 0) > 0 && grpcPanelOpen" class="panel swagger-diff-panel grpc-diff-panel">
        <button
          class="icon-action-button secondary swagger-close-icon"
          type="button"
          title="Close gRPC Reflection Diff"
          aria-label="Close gRPC Reflection Diff"
          @click="closeGrpcPanel"
        >
          <span class="icon-action-glyph">✕</span>
        </button>
        <div class="page-header endpoints-head swagger-panel-head">
          <h3>gRPC Reflection Diff</h3>
        </div>
        <p v-if="grpcDiffLoading" class="muted">Loading gRPC reflection methods...</p>
        <p v-else-if="grpcDiffError" class="error">{{ grpcDiffError }}</p>
        <template v-else>
          <div v-if="grpcRegisteredInvalid.length > 0" class="swagger-invalid-toggle-wrap">
            <button class="secondary-button swagger-invalid-toggle-button" type="button" @click="toggleGrpcRegisteredInvalid">
              {{
                grpcRegisteredInvalidOpen
                  ? `Hide Registered Invalid (${grpcRegisteredInvalid.length})`
                  : `Show Registered Invalid (${grpcRegisteredInvalid.length})`
              }}
            </button>
          </div>
          <div class="swagger-diff-grid">
            <div v-if="grpcRegisteredInvalidOpen" class="swagger-diff-column">
              <div class="swagger-diff-title-row">
                <div class="label">Registered Invalid</div>
                <span class="swagger-diff-count">{{ grpcRegisteredInvalid.length }}</span>
              </div>
              <div class="swagger-section-box">
                <div v-if="grpcRegisteredInvalidGroups.length > 0" class="swagger-endpoint-groups">
                  <section
                    v-for="group in grpcRegisteredInvalidGroups"
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
                <div v-if="grpcUnregistered.length > 0" class="swagger-bulk-actions">
                  <button
                    class="primary-button swagger-bulk-add-button"
                    type="button"
                    :disabled="grpcBulkAdding || grpcSelectedCount === 0"
                    @click="addSelectedGrpcEndpoints"
                  >
                    {{ grpcBulkAdding ? "Adding..." : `Add selected (${grpcSelectedCount})` }}
                  </button>
                </div>
                <div v-if="grpcUnregisteredGroups.length > 0" class="swagger-endpoint-groups">
                  <section v-for="group in grpcUnregisteredGroups" :key="`grpc-missing-group-${group.segment}`" class="swagger-endpoint-group">
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
                        <label class="swagger-item-select">
                          <input
                            type="checkbox"
                            :checked="isGrpcSelected(item)"
                            :disabled="grpcBulkAdding"
                            @change="onGrpcItemSelectChange(item, $event)"
                          />
                        </label>
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
                        >
                          <span class="swagger-add-plus">+</span>
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
      </section>
    </Transition>

    <div class="page-header compact endpoints-head">
      <h3>{{ activeEndpointType === "grpc" ? "gRPC Endpoints" : "HTTP Endpoints" }}</h3>
      <span class="muted endpoints-head-count">{{ filteredEndpoints.length }} / {{ activeProtocolTotal }}</span>
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
      <select v-if="activeEndpointType === 'http'" v-model="httpMethodFilter" class="endpoints-filter-select" aria-label="Filter by HTTP method">
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
              :disabled="deletingApp || deletingEndpointId !== '' || togglingApp"
              title="Delete Endpoint"
              aria-label="Delete Endpoint"
              @click="removeEndpoint(endpoint)"
            >
              <span class="icon-action-glyph">{{ deletingEndpointId === endpoint.id ? "…" : "🗑" }}</span>
            </button>
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
              <span class="status-chip" :class="{ inactive: !endpoint.active }">
                {{ endpoint.active ? "active" : "inactive" }}
              </span>
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
                <span class="icon-action-glyph">{{ deletingEndpointId === endpoint.id ? "…" : "🗑" }}</span>
              </button>
            </div>
          </article>
        </div>
      </section>
    </div>
  </template>
</template>
