<script setup lang="ts">
import { computed, onMounted, ref, type Component } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { useDialog } from "naive-ui";
import { ArrowBackOutline, BanOutline, CopyOutline, CreateOutline, GlobeOutline, KeyOutline, LockClosedOutline, PersonOutline, TrashOutline } from "@vicons/ionicons5";
import { deleteEndpoint, getApp, getEndpoint, getRoot, updateEndpoint } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppMain, EndpointMain } from "../types/api";

type EndpointAuthIcon = {
  key: "ip_validation" | "jwt" | "basic" | "api_key";
  component: Component;
  label: string;
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
const app = ref<AppMain | null>(null);
const rootBaseUrl = ref("");
const appName = ref("");

const endpointType = computed(() => (endpoint.value?.type === "grpc" ? "grpc" : "http"));
const endpointPath = computed(() => {
  if (endpointType.value === "grpc") {
    return normalizedRoutePath(endpoint.value?.grpc?.path || endpoint.value?.http?.path || "");
  }
  return normalizedRoutePath(endpoint.value?.http?.path || "");
});
const endpointMethod = computed(() => (endpointType.value === "grpc" ? "GRPC" : (endpoint.value?.http?.method || "").trim().toUpperCase() || "*"));
const publicRoute = computed(() => {
  if (endpointType.value === "grpc") {
    return endpointPath.value;
  }
  if (!app.value || !rootBaseUrl.value) {
    return "";
  }
  const routePath = joinPathParts(app.value.path_prefix, endpoint.value?.http?.path || "");
  return joinUrl(rootBaseUrl.value, routePath);
});
const backendUrl = computed(() => {
  if (!app.value) {
    return "";
  }
  if (endpointType.value === "grpc") {
    return grpcBackendAddress(app.value);
  }
  const targetPath = endpoint.value?.backend.custom_path || endpoint.value?.http?.path || "";
  return joinUrl(app.value.backend.url, targetPath);
});

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

function endpointAuthIcons(item: EndpointMain): EndpointAuthIcon[] {
  const methods = item.auth?.methods || [];
  const hasIpValidation = methods.some((method) => Boolean(method.ip_validation));
  const hasJwt = methods.some((method) => Boolean(method.jwt));
  const hasBasic = methods.some((method) => Boolean(method.basic));
  const hasApiKey = methods.some((method) => Boolean(method.api_key));

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

function authSummary(item: EndpointMain): string {
  if (!item.auth?.enabled) {
    return "Public access (auth disabled)";
  }
  const mode = (item.auth.mode || "extend").toLowerCase() === "replace" ? "replace" : "extend";
  return `Auth enabled, mode: ${mode}`;
}

async function load() {
  loading.value = true;
  errorMessage.value = "";
  try {
    endpoint.value = await getEndpoint(id.value);
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
    content: `Deactivate endpoint ${endpointMethod.value} ${endpointPath.value}?`,
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
    content: `Delete endpoint ${endpointMethod.value} ${endpointPath.value}?`,
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
  </div>

  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
  <p v-if="loading" class="muted">Loading...</p>
  <p v-else-if="!endpoint" class="muted">Endpoint not found.</p>

  <template v-else>
    <section class="endpoint-details-hero">
      <div class="endpoint-details-main">
        <span class="http-method-badge" :class="endpointMethodBadgeClass(endpointMethod)">{{ endpointMethod }}</span>
        <h2 class="endpoint-details-path">{{ endpointPath }}</h2>
      </div>
      <n-tag size="small" :type="endpoint.active ? 'success' : 'warning'">
        {{ endpoint.active ? "active" : "inactive" }}
      </n-tag>
    </section>

    <section class="summary-grid endpoint-summary-grid">
      <div>
        <span class="label">Protocol</span>
        <strong>{{ endpointType === "grpc" ? "gRPC" : "HTTP" }}</strong>
      </div>
      <div>
        <span class="label">Application</span>
        <strong>{{ appName || endpoint.app_id }}</strong>
      </div>
      <div>
        <span class="label">App ID</span>
        <strong>{{ endpoint.app_id }}</strong>
      </div>
      <div>
        <span class="label">Endpoint ID</span>
        <strong class="endpoint-value-break">{{ endpoint.id }}</strong>
      </div>
      <div v-if="endpointType === 'http'">
        <span class="label">Custom Backend Path</span>
        <strong>{{ endpoint.backend.custom_path || "inherit app backend path" }}</strong>
      </div>
      <div v-if="endpointType === 'grpc'">
        <span class="label">gRPC Service</span>
        <strong>{{ endpoint.grpc.service }}</strong>
      </div>
      <div v-if="endpointType === 'grpc'">
        <span class="label">gRPC Method</span>
        <strong>{{ endpoint.grpc.method }}</strong>
      </div>
      <div>
        <span class="label">{{ endpointType === "grpc" ? "gRPC Path" : "Public URL" }}</span>
        <button
          class="endpoint-copy-link"
          type="button"
          :disabled="!publicRoute"
          :title="endpointType === 'grpc' ? 'Copy gRPC Path' : 'Copy Public URL'"
          :aria-label="endpointType === 'grpc' ? 'Copy gRPC Path' : 'Copy Public URL'"
          @click="copyUrl(endpointType === 'grpc' ? 'gRPC Path' : 'Public URL', publicRoute)"
        >
          <span class="endpoint-copy-value" :class="{ muted: !publicRoute }">{{ publicRoute || "unavailable" }}</span>
          <n-icon class="endpoint-copy-icon" :component="CopyOutline" aria-hidden="true" />
        </button>
      </div>
      <div>
        <span class="label">{{ endpointType === "grpc" ? "Backend gRPC Address" : "Backend URL" }}</span>
        <button
          class="endpoint-copy-link"
          type="button"
          :disabled="!backendUrl"
          :title="endpointType === 'grpc' ? 'Copy Backend gRPC Address' : 'Copy Backend URL'"
          :aria-label="endpointType === 'grpc' ? 'Copy Backend gRPC Address' : 'Copy Backend URL'"
          @click="copyUrl(endpointType === 'grpc' ? 'Backend gRPC Address' : 'Backend URL', backendUrl)"
        >
          <span class="endpoint-copy-value" :class="{ muted: !backendUrl }">{{ backendUrl || "unavailable" }}</span>
          <n-icon class="endpoint-copy-icon" :component="CopyOutline" aria-hidden="true" />
        </button>
      </div>
    </section>

    <section class="panel endpoint-auth-panel">
      <div class="endpoint-auth-head">
        <h3>Auth</h3>
        <span v-if="endpoint.auth?.enabled" class="endpoint-lock-chip" title="Auth required" aria-label="Auth required">
          <n-icon :component="LockClosedOutline" />
        </span>
      </div>
      <p class="muted endpoint-auth-summary">{{ authSummary(endpoint) }}</p>
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
      <p v-else class="muted">No auth methods configured.</p>
    </section>
  </template>
</template>
