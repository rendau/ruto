<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { useDialog } from "naive-ui";
import { ArrowBackOutline, BanOutline, CopyOutline, CreateOutline, EyeOutline, GitNetworkOutline, TrashOutline } from "@vicons/ionicons5";
import AuthCard from "../components/AuthCard.vue";
import { deleteEndpoint, getApp, getEndpoint, getEndpointInherited, getEndpointInterpolate, getRoot, updateEndpoint } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppMain, EndpointMain } from "../types/api";

type EndpointCardField = {
  key: string;
  label: string;
  value: string;
  multiline?: boolean;
  copyLabel?: string;
};

type EndpointCardView = {
  method: string;
  path: string;
  active: boolean;
  fields: EndpointCardField[];
};

const route = useRoute();
const router = useRouter();
const dialog = useDialog();

const id = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));
const loading = ref(false);
const deactivating = ref(false);
const deleting = ref(false);
const errorMessage = ref("");

const endpoint = ref<EndpointMain | null>(null);
const endpointInherited = ref<EndpointMain | null>(null);
const endpointInheritedError = ref("");
const app = ref<AppMain | null>(null);
const rootBaseUrl = ref("");
const appName = ref("");
const interpolateModalVisible = ref(false);
const interpolatedLoading = ref(false);
const interpolatedError = ref("");
const endpointInterpolated = ref<EndpointMain | null>(null);
const inheritedExpandedNames = ref<string[]>([]);

const currentCard = computed(() => buildEndpointCard(endpoint.value));
const inheritedCard = computed(() => buildEndpointCard(endpointInherited.value));
const interpolatedCard = computed(() => buildEndpointCard(endpointInterpolated.value));

function normalizedRoutePath(path: string): string {
  const trimmed = (path || "").trim();
  if (!trimmed) {
    return "/";
  }
  return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
}

function cleanPathPart(part: string): string {
  return (part || "").trim().replace(/^\/+|\/+$/g, "");
}

function joinPathParts(...parts: string[]): string {
  return parts
    .map((part) => cleanPathPart(part))
    .filter((part) => part.length > 0)
    .join("/");
}

function joinUrl(baseUrl: string, path: string): string {
  const base = (baseUrl || "").trim().replace(/\/+$/g, "");
  if (!base) {
    return "";
  }
  const cleanedPath = cleanPathPart(path);
  return cleanedPath ? `${base}/${cleanedPath}` : base;
}

function grpcBackendAddress(item: AppMain): string {
  return (item.backend.grpc_url || "").trim();
}

function endpointTypeOf(item: EndpointMain): "grpc" | "http" {
  return item.type === "grpc" ? "grpc" : "http";
}

function endpointPathOf(item: EndpointMain): string {
  if (endpointTypeOf(item) === "grpc") {
    return normalizedRoutePath(item.grpc.path || item.http.path || "");
  }
  return normalizedRoutePath(item.http.path || "");
}

function endpointMethodOf(item: EndpointMain): string {
  if (endpointTypeOf(item) === "grpc") {
    return "GRPC";
  }
  return (item.http.method || "").trim().toUpperCase() || "*";
}

function endpointPublicRoute(item: EndpointMain): string {
  if (endpointTypeOf(item) === "grpc") {
    return endpointPathOf(item);
  }
  if (!app.value || !rootBaseUrl.value) {
    return "";
  }
  const routePath = joinPathParts(app.value.path_prefix, item.http.path || "");
  return joinUrl(rootBaseUrl.value, routePath);
}

function endpointBackendRoute(item: EndpointMain): string {
  if (!app.value) {
    return "";
  }
  if (endpointTypeOf(item) === "grpc") {
    return grpcBackendAddress(app.value);
  }
  const targetPath = item.backend.custom_path || item.http.path || "";
  return joinUrl(app.value.backend.url, targetPath);
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

function recordRows(record: Record<string, string> | undefined): string {
  const entries = Object.entries(record || {});
  if (entries.length === 0) {
    return "none";
  }
  return entries.map(([key, value]) => `${key}: ${value}`).join("\n");
}

function variableRows(item: EndpointMain): string {
  if (!item.variables || item.variables.length === 0) {
    return "none";
  }
  return item.variables.map((entry) => `${entry.key}: ${entry.value}`).join("\n");
}

function buildEndpointCard(item: EndpointMain | null): EndpointCardView | null {
  if (!item) {
    return null;
  }

  const type = endpointTypeOf(item);
  const publicRoute = endpointPublicRoute(item) || "unavailable";
  const backendRoute = endpointBackendRoute(item) || "unavailable";

  const fields: EndpointCardField[] = [
    { key: "protocol", label: "Protocol", value: type === "grpc" ? "gRPC" : "HTTP" },
    { key: "application", label: "Application", value: appName.value || item.app_id || "-" },
    { key: "app-id", label: "App ID", value: item.app_id || "-" },
    { key: "endpoint-id", label: "Endpoint ID", value: item.id || "-", multiline: true }
  ];

  if (type === "grpc") {
    fields.push(
      { key: "grpc-service", label: "gRPC Service", value: item.grpc.service || "-" },
      { key: "grpc-method", label: "gRPC Method", value: item.grpc.method || "-" }
    );
  } else {
    fields.push({
      key: "custom-path",
      label: "Custom Backend Path",
      value: item.backend.custom_path || "inherit app backend path"
    });
  }

  fields.push(
    {
      key: "public-route",
      label: type === "grpc" ? "gRPC Path" : "Public URL",
      value: publicRoute,
      copyLabel: type === "grpc" ? "gRPC Path" : "Public URL"
    },
    { key: "backend-headers", label: "Backend Headers", value: recordRows(item.backend.headers), multiline: true },
    {
      key: "backend-route",
      label: type === "grpc" ? "Backend gRPC Address" : "Backend URL",
      value: backendRoute,
      copyLabel: type === "grpc" ? "Backend gRPC Address" : "Backend URL"
    },
    { key: "backend-query", label: "Backend Query Params", value: recordRows(item.backend.query_params), multiline: true },
    { key: "variables", label: "Variables", value: variableRows(item), multiline: true }
  );

  return {
    method: endpointMethodOf(item),
    path: endpointPathOf(item),
    active: Boolean(item.active),
    fields
  };
}

function canCopyField(field: EndpointCardField): boolean {
  return Boolean(field.copyLabel && field.value && field.value !== "unavailable");
}

async function copyField(field: EndpointCardField) {
  if (!field.copyLabel) {
    return;
  }
  await copyUrl(field.copyLabel, canCopyField(field) ? field.value : "");
}

async function loadInherited(item: EndpointMain) {
  endpointInheritedError.value = "";
  endpointInherited.value = null;
  try {
    endpointInherited.value = await getEndpointInherited({
      id: item.id,
      app_id: item.app_id,
      variables: item.variables || []
    });
  } catch (error) {
    endpointInheritedError.value = error instanceof Error ? error.message : "Unable to load inherited values";
  }
}

async function openInterpolatedModal() {
  if (!endpoint.value || interpolatedLoading.value) {
    return;
  }
  interpolateModalVisible.value = true;
  interpolatedLoading.value = true;
  interpolatedError.value = "";
  endpointInterpolated.value = null;
  try {
    endpointInterpolated.value = await getEndpointInterpolate({
      id: endpoint.value.id,
      app_id: endpoint.value.app_id,
      variables: endpoint.value.variables || []
    });
  } catch (error) {
    interpolatedError.value = error instanceof Error ? error.message : "Unable to load interpolated values";
  } finally {
    interpolatedLoading.value = false;
  }
}

async function load() {
  loading.value = true;
  errorMessage.value = "";
  try {
    endpoint.value = await getEndpoint(id.value);
    await loadInherited(endpoint.value);
    app.value = null;
    rootBaseUrl.value = "";
    appName.value = "";
    try {
      const root = await getRoot();
      rootBaseUrl.value = root.base_url || "";
    } catch {
      rootBaseUrl.value = "";
    }
    if (endpoint.value.app_id) {
      try {
        app.value = await getApp(endpoint.value.app_id);
        appName.value = app.value.name || app.value.id;
      } catch {
        app.value = null;
        appName.value = "";
      }
    }
  } catch (error) {
    endpoint.value = null;
    endpointInherited.value = null;
    endpointInheritedError.value = "";
    errorMessage.value = error instanceof Error ? error.message : "Unable to load endpoint";
  } finally {
    loading.value = false;
  }
}

async function deactivateEndpoint() {
  if (deactivating.value || deleting.value || !endpoint.value || !endpoint.value.active) {
    return;
  }
  dialog.warning({
    title: "Deactivate endpoint",
    content: `Deactivate endpoint ${endpointMethodOf(endpoint.value)} ${endpointPathOf(endpoint.value)}?`,
    positiveText: "Deactivate",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runDeactivateEndpoint();
    }
  });
}

async function runDeactivateEndpoint() {
  if (!endpoint.value) {
    return;
  }
  deactivating.value = true;
  errorMessage.value = "";
  try {
    await updateEndpoint({
      ...endpoint.value,
      active: false
    });
    endpoint.value.active = false;
    notifySuccess("Endpoint deactivated");
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to deactivate endpoint";
    notifyError(errorMessage.value);
  } finally {
    deactivating.value = false;
  }
}

async function removeEndpoint() {
  if (deleting.value || deactivating.value || !endpoint.value) {
    return;
  }
  dialog.error({
    title: "Delete endpoint",
    content: `Delete endpoint ${endpointMethodOf(endpoint.value)} ${endpointPathOf(endpoint.value)}?`,
    positiveText: "Delete",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runRemoveEndpoint();
    }
  });
}

async function runRemoveEndpoint() {
  if (!endpoint.value) {
    return;
  }
  deleting.value = true;
  errorMessage.value = "";
  try {
    const appId = endpoint.value.app_id;
    await deleteEndpoint(endpoint.value.id);
    notifySuccess("Endpoint deleted");
    await router.push({ name: "app-details", params: { id: appId } });
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to delete endpoint";
    notifyError(errorMessage.value);
  } finally {
    deleting.value = false;
  }
}

async function copyUrl(label: string, value: string) {
  if (!value) {
    return;
  }
  try {
    await navigator.clipboard.writeText(value);
    notifySuccess(`${label} copied`);
  } catch {
    notifyError(`Unable to copy ${label.toLowerCase()}`);
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="actions page-top-actions endpoint-details-actions">
    <RouterLink
      v-if="endpoint"
      class="icon-action-button secondary"
      :to="{ name: 'app-details', params: { id: endpoint.app_id } }"
      title="Back to Application"
      aria-label="Back to Application"
    >
      <n-icon :component="ArrowBackOutline" />
    </RouterLink>
    <RouterLink
      v-if="endpoint"
      class="icon-action-button secondary"
      :to="{ name: 'endpoint-edit', params: { id: endpoint.id } }"
      title="Edit Endpoint"
      aria-label="Edit Endpoint"
    >
      <n-icon :component="CreateOutline" />
    </RouterLink>
    <n-button
      v-if="endpoint"
      type="primary"
      secondary
      :disabled="deactivating || deleting || !endpoint.active"
      :loading="deactivating"
      :title="endpoint.active ? 'Deactivate Endpoint' : 'Endpoint already inactive'"
      aria-label="Deactivate Endpoint"
      @click="deactivateEndpoint"
    >
      <n-icon :component="BanOutline" />
    </n-button>
    <n-button
      v-if="endpoint"
      class="danger-icon-button"
      type="error"
      secondary
      circle
      :disabled="deleting || deactivating"
      :loading="deleting"
      title="Delete Endpoint"
      aria-label="Delete Endpoint"
      @click="removeEndpoint"
    >
      <n-icon v-if="!deleting" :component="TrashOutline" />
    </n-button>
    <n-button
      v-if="endpoint"
      title="Show interpolated values"
      aria-label="Show interpolated values"
      @click="openInterpolatedModal"
    >
      <n-icon :component="EyeOutline" />
    </n-button>
  </div>

  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
  <p v-if="loading" class="muted">Loading...</p>
  <p v-else-if="!endpoint" class="muted">Endpoint not found.</p>

  <template v-else>
    <section v-if="currentCard" class="panel endpoint-card">
      <div class="panel-head">
        <h3 class="endpoint-card-title">
          <n-icon :component="GitNetworkOutline" />
          <span>Endpoint</span>
        </h3>
      </div>
      <div class="endpoint-details-hero">
        <div class="endpoint-details-main">
          <span class="http-method-badge" :class="endpointMethodBadgeClass(currentCard.method)">{{ currentCard.method }}</span>
          <h2 class="endpoint-details-path">{{ currentCard.path }}</h2>
        </div>
        <n-tag size="small" :type="currentCard.active ? 'success' : 'warning'">
          {{ currentCard.active ? "active" : "inactive" }}
        </n-tag>
      </div>
      <div class="summary-grid endpoint-summary-grid">
        <div v-for="field in currentCard.fields" :key="`current-${field.key}`">
          <span class="label">{{ field.label }}</span>
          <button
            v-if="field.copyLabel"
            class="endpoint-copy-link"
            type="button"
            :disabled="!canCopyField(field)"
            :title="`Copy ${field.copyLabel}`"
            :aria-label="`Copy ${field.copyLabel}`"
            @click="copyField(field)"
          >
            <span class="endpoint-copy-value endpoint-value-break" :class="{ muted: !canCopyField(field), 'endpoint-multiline': field.multiline }">
              {{ field.value }}
            </span>
            <n-icon class="endpoint-copy-icon" :component="CopyOutline" aria-hidden="true" />
          </button>
          <strong v-else class="endpoint-value-break" :class="{ 'endpoint-multiline': field.multiline }">{{ field.value }}</strong>
        </div>
      </div>
      <AuthCard :auth="endpoint?.auth" />
    </section>

    <section class="panel endpoint-card">
      <n-collapse v-model:expanded-names="inheritedExpandedNames" class="endpoint-inherited-collapse">
        <n-collapse-item name="inherited" display-directive="if">
          <template #header>
            <span class="endpoint-card-title">
              <n-icon :component="GitNetworkOutline" />
              <span>Endpoint (Inherited)</span>
            </span>
          </template>

          <p v-if="endpointInheritedError" class="muted">{{ endpointInheritedError }}</p>
          <template v-else-if="inheritedCard">
            <div class="endpoint-details-hero">
              <div class="endpoint-details-main">
                <span class="http-method-badge" :class="endpointMethodBadgeClass(inheritedCard.method)">{{ inheritedCard.method }}</span>
                <h2 class="endpoint-details-path">{{ inheritedCard.path }}</h2>
              </div>
              <n-tag size="small" :type="inheritedCard.active ? 'success' : 'warning'">
                {{ inheritedCard.active ? "active" : "inactive" }}
              </n-tag>
            </div>
            <div class="summary-grid endpoint-summary-grid">
              <div v-for="field in inheritedCard.fields" :key="`inherited-${field.key}`">
                <span class="label">{{ field.label }}</span>
                <button
                  v-if="field.copyLabel"
                  class="endpoint-copy-link"
                  type="button"
                  :disabled="!canCopyField(field)"
                  :title="`Copy ${field.copyLabel}`"
                  :aria-label="`Copy ${field.copyLabel}`"
                  @click="copyField(field)"
                >
                  <span class="endpoint-copy-value endpoint-value-break" :class="{ muted: !canCopyField(field), 'endpoint-multiline': field.multiline }">
                    {{ field.value }}
                  </span>
                  <n-icon class="endpoint-copy-icon" :component="CopyOutline" aria-hidden="true" />
                </button>
                <strong v-else class="endpoint-value-break" :class="{ 'endpoint-multiline': field.multiline }">{{ field.value }}</strong>
              </div>
            </div>
            <AuthCard :auth="endpointInherited?.auth" />
          </template>
          <p v-else class="muted">Inherited values unavailable.</p>
        </n-collapse-item>
      </n-collapse>
    </section>

    <n-modal
      v-model:show="interpolateModalVisible"
      preset="card"
      title="Interpolated Endpoint"
      class="endpoint-modal-card"
      :bordered="false"
      :mask-closable="true"
      content-style="display: flex; min-height: 0; height: 100%; overflow: hidden;"
    >
      <div class="endpoint-modal-content">
        <p v-if="interpolatedLoading" class="muted">Loading interpolated values...</p>
        <n-alert v-else-if="interpolatedError" type="error" :show-icon="false">{{ interpolatedError }}</n-alert>
        <section v-else-if="interpolatedCard" class="endpoint-card endpoint-card-modal">
          <div class="endpoint-details-hero">
            <div class="endpoint-details-main">
              <span class="http-method-badge" :class="endpointMethodBadgeClass(interpolatedCard.method)">{{ interpolatedCard.method }}</span>
              <h2 class="endpoint-details-path">{{ interpolatedCard.path }}</h2>
            </div>
            <n-tag size="small" :type="interpolatedCard.active ? 'success' : 'warning'">
              {{ interpolatedCard.active ? "active" : "inactive" }}
            </n-tag>
          </div>
          <div class="summary-grid endpoint-summary-grid">
            <div v-for="field in interpolatedCard.fields" :key="`interpolate-${field.key}`">
              <span class="label">{{ field.label }}</span>
              <button
                v-if="field.copyLabel"
                class="endpoint-copy-link"
                type="button"
                :disabled="!canCopyField(field)"
                :title="`Copy ${field.copyLabel}`"
                :aria-label="`Copy ${field.copyLabel}`"
                @click="copyField(field)"
              >
                <span class="endpoint-copy-value endpoint-value-break" :class="{ muted: !canCopyField(field), 'endpoint-multiline': field.multiline }">
                  {{ field.value }}
                </span>
                <n-icon class="endpoint-copy-icon" :component="CopyOutline" aria-hidden="true" />
              </button>
              <strong v-else class="endpoint-value-break" :class="{ 'endpoint-multiline': field.multiline }">{{ field.value }}</strong>
            </div>
          </div>
          <AuthCard :auth="endpointInterpolated?.auth" />
        </section>
        <p v-else class="muted">Interpolated values unavailable.</p>
      </div>
    </n-modal>
  </template>
</template>
