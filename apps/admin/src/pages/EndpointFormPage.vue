<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createEndpoint, getApp, getAppGrpcReflectionEndpoints, getEndpoint, getRoot, getRootJwtKidsByUrls, updateEndpoint } from "../lib/api";
import AuthEditor from "../components/AuthEditor.vue";
import { normalizeAuth } from "../lib/forms";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppGrpcReflectionEndpoint, EndpointMain, EndpointType } from "../types/api";

const route = useRoute();
const router = useRouter();

const isEdit = computed(() => typeof route.params.id === "string" && route.params.id.length > 0);
const endpointId = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));
const appIdFromRoute = computed(() => (typeof route.params.appId === "string" ? route.params.appId : ""));
const prefillTypeFromQuery = computed<EndpointType>(() => (route.query.type === "grpc" ? "grpc" : "http"));
const prefillMethodFromQuery = computed(() =>
  typeof route.query.method === "string" ? route.query.method.trim().toUpperCase() : ""
);
const prefillPathFromQuery = computed(() => (typeof route.query.path === "string" ? route.query.path.trim() : ""));
const prefillGrpcServiceFromQuery = computed(() =>
  typeof route.query.grpc_service === "string" ? route.query.grpc_service.trim() : ""
);
const prefillGrpcMethodFromQuery = computed(() =>
  typeof route.query.grpc_method === "string" ? route.query.grpc_method.trim() : ""
);
const prefillGrpcPathFromQuery = computed(() =>
  typeof route.query.grpc_path === "string" ? route.query.grpc_path.trim() : ""
);

const loading = ref(false);
const saving = ref(false);
const errorMessage = ref("");
const appName = ref("");
const jwtKidOptions = ref<string[]>([]);
const lastAutoGrpcPath = ref("");
const appGrpcEnabled = ref(false);
const grpcReflectionLoading = ref(false);
const grpcReflectionError = ref("");
const grpcReflectionOptions = ref<AppGrpcReflectionEndpoint[]>([]);
const selectedGrpcReflectionPath = ref("");

const form = ref<EndpointMain>({
  id: "",
  app_id: appIdFromRoute.value,
  active: true,
  method: "GET",
  path: "",
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
});
const appDisplayName = computed(() => appName.value || form.value.app_id || "-");
const endpointMethodOptions = ["*", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE"];
const protocolOptions = computed(() => {
  const options: Array<{ value: EndpointType; label: string }> = [{ value: "http", label: "HTTP" }];
  if (appGrpcEnabled.value || (isEdit.value && form.value.type === "grpc")) {
    options.push({ value: "grpc", label: "gRPC" });
  }
  return options;
});
const grpcReflectionAvailable = computed(() => grpcReflectionOptions.value.length > 0);
const grpcReflectionDisabledReason = computed(() => {
  if (appGrpcEnabled.value) {
    return "";
  }
  return "gRPC reflection is available only when app gRPC Port is enabled (> 0).";
});

function normalizeLoadedEndpoint(item: EndpointMain): EndpointMain {
  const endpointType: EndpointType = item.type === "grpc" ? "grpc" : "http";
  return {
    ...item,
    type: endpointType,
    backend: {
      custom_path: item.backend?.custom_path || ""
    },
    auth: normalizeAuth(item.auth),
    grpc: {
      service: item.grpc?.service || "",
      method: item.grpc?.method || "",
      path: item.grpc?.path || ""
    }
  };
}

function applyPrefillFromQuery() {
  form.value.type = prefillTypeFromQuery.value;
  if (form.value.type === "grpc") {
    form.value.grpc.service = prefillGrpcServiceFromQuery.value;
    form.value.grpc.method = prefillGrpcMethodFromQuery.value;
    form.value.grpc.path = normalizeGrpcPath(
      prefillGrpcServiceFromQuery.value,
      prefillGrpcMethodFromQuery.value,
      prefillGrpcPathFromQuery.value
    );
    if (form.value.grpc.path) {
      lastAutoGrpcPath.value = form.value.grpc.path;
    }
  } else {
    if (prefillMethodFromQuery.value && endpointMethodOptions.includes(prefillMethodFromQuery.value)) {
      form.value.method = prefillMethodFromQuery.value;
    }
    if (prefillPathFromQuery.value) {
      form.value.path = prefillPathFromQuery.value;
    }
  }
}

async function loadAppName() {
  if (!form.value.app_id) {
    appName.value = "";
    appGrpcEnabled.value = false;
    grpcReflectionOptions.value = [];
    grpcReflectionError.value = "";
    return;
  }
  try {
    const app = await getApp(form.value.app_id);
    appName.value = app.name;
    appGrpcEnabled.value = Number(app.backend.grpc_port || 0) > 0;
    if (appGrpcEnabled.value) {
      await loadGrpcReflectionOptions();
    } else {
      grpcReflectionOptions.value = [];
      grpcReflectionError.value = "";
    }
  } catch {
    appName.value = "";
    appGrpcEnabled.value = false;
    grpcReflectionOptions.value = [];
    grpcReflectionError.value = "";
  }
}

async function loadGrpcReflectionOptions() {
  if (!form.value.app_id || !appGrpcEnabled.value) {
    grpcReflectionOptions.value = [];
    grpcReflectionLoading.value = false;
    if (!appGrpcEnabled.value) {
      grpcReflectionError.value = "";
    }
    return;
  }
  grpcReflectionLoading.value = true;
  grpcReflectionError.value = "";
  try {
    const rep = await getAppGrpcReflectionEndpoints(form.value.app_id);
    grpcReflectionOptions.value = rep.results || [];
    syncSelectedGrpcReflectionPath();
  } catch (error) {
    grpcReflectionOptions.value = [];
    grpcReflectionError.value = error instanceof Error ? error.message : "gRPC reflection unavailable";
  } finally {
    grpcReflectionLoading.value = false;
  }
}

async function load() {
  if (!isEdit.value) {
    applyPrefillFromQuery();
    await loadAppName();
    return;
  }
  loading.value = true;
  errorMessage.value = "";
  try {
    form.value = normalizeLoadedEndpoint(await getEndpoint(endpointId.value));
    await loadAppName();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load endpoint";
  } finally {
    loading.value = false;
  }
}

async function loadJwtKidOptions() {
  try {
    const root = await getRoot();
    const rep = await getRootJwtKidsByUrls({
      urls: (root.jwt || []).map((item) => item.jwk_url).filter(Boolean)
    });
    jwtKidOptions.value = rep.kids || [];
  } catch {
    jwtKidOptions.value = [];
  }
}

function normalizeGrpcPath(service: string, method: string, path: string): string {
  const explicitPath = path.trim();
  if (explicitPath) {
    return explicitPath.startsWith("/") ? explicitPath : `/${explicitPath}`;
  }

  const cleanService = service.trim();
  const cleanMethod = method.trim();
  if (!cleanService || !cleanMethod) {
    return "";
  }
  return `/${cleanService}/${cleanMethod}`;
}

function grpcReflectionOptionLabel(option: AppGrpcReflectionEndpoint): string {
  return `${option.service}/${option.method}`;
}

function syncSelectedGrpcReflectionPath() {
  const currentPath = normalizeGrpcPath(form.value.grpc.service, form.value.grpc.method, form.value.grpc.path);
  const matched = grpcReflectionOptions.value.find((option) => option.path === currentPath);
  selectedGrpcReflectionPath.value = matched?.path || "";
}

function applyGrpcReflectionOption(path: string) {
  const selected = grpcReflectionOptions.value.find((option) => option.path === path);
  if (!selected) {
    return;
  }
  form.value.grpc.service = selected.service;
  form.value.grpc.method = selected.method;
  form.value.grpc.path = selected.path;
  lastAutoGrpcPath.value = selected.path;
  selectedGrpcReflectionPath.value = selected.path;
}

function buildPayload(): EndpointMain {
  const endpointType: EndpointType = form.value.type === "grpc" ? "grpc" : "http";
  const payload: EndpointMain = {
    ...form.value,
    type: endpointType,
    backend: {
      custom_path: form.value.backend?.custom_path || ""
    },
    auth: normalizeAuth(form.value.auth),
    grpc: {
      service: form.value.grpc?.service || "",
      method: form.value.grpc?.method || "",
      path: form.value.grpc?.path || ""
    }
  };

  if (payload.type === "grpc") {
    payload.grpc = {
      service: payload.grpc.service.trim(),
      method: payload.grpc.method.trim(),
      path: normalizeGrpcPath(payload.grpc.service, payload.grpc.method, payload.grpc.path)
    };
    payload.method = "GRPC";
    payload.path = payload.grpc.path;
    payload.backend.custom_path = "";
    return payload;
  }

  payload.method = (payload.method || "").trim().toUpperCase();
  payload.path = (payload.path || "").trim();
  payload.grpc = {
    service: "",
    method: "",
    path: ""
  };
  return payload;
}

watch(protocolOptions, (options) => {
  if (!options.find((o) => o.value === form.value.type)) {
    form.value.type = "http";
  }
});

async function submit() {
  if (form.value.type === "grpc" && !appGrpcEnabled.value) {
    errorMessage.value = "Cannot save gRPC endpoint: application gRPC port is not configured.";
    notifyError(errorMessage.value);
    return;
  }
  saving.value = true;
  errorMessage.value = "";
  const payload = buildPayload();
  try {
    if (isEdit.value) {
      await updateEndpoint(payload);
      notifySuccess("Endpoint updated");
      router.back();
      return;
    }
    await createEndpoint(payload);
    notifySuccess("Endpoint created");
    router.back();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to save endpoint";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}

watch(
  () => [form.value.grpc.service, form.value.grpc.method],
  () => {
    if (form.value.type !== "grpc") {
      return;
    }
    const currentPath = form.value.grpc.path.trim();
    if (currentPath && currentPath !== lastAutoGrpcPath.value) {
      return;
    }
    const nextPath = normalizeGrpcPath(form.value.grpc.service, form.value.grpc.method, "");
    form.value.grpc.path = nextPath;
    lastAutoGrpcPath.value = nextPath;
    syncSelectedGrpcReflectionPath();
  }
);

watch(
  () => form.value.grpc.path,
  () => {
    if (form.value.type === "grpc") {
      syncSelectedGrpcReflectionPath();
    }
  }
);

onMounted(() => {
  void Promise.all([load(), loadJwtKidOptions()]);
});
</script>

<template>
  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submit">
    <div class="field">
      <span>Application</span>
      <div class="field-readonly">{{ appDisplayName }}</div>
    </div>
    <label class="check">
      <input v-model="form.active" type="checkbox" />
      <span>Active</span>
    </label>
    <div class="field">
      <span>Protocol</span>
      <div class="protocol-tabs form-protocol-tabs" role="tablist" aria-label="Endpoint Protocol">
        <button
          v-for="option in protocolOptions"
          :key="option.value"
          class="protocol-tab"
          :class="{ active: form.type === option.value }"
          type="button"
          :aria-selected="form.type === option.value"
          @click="form.type = option.value"
        >
          {{ option.label }}
        </button>
      </div>
    </div>

    <template v-if="form.type === 'http'">
      <label class="field">
        <span>Method</span>
        <select v-model="form.method" required>
          <option v-for="method in endpointMethodOptions" :key="method" :value="method">
            {{ method }}
          </option>
        </select>
      </label>
      <label class="field">
        <span>Path</span>
        <input v-model="form.path" placeholder="/path or empty for app root" />
      </label>
      <label class="field">
        <span>Custom Backend Path</span>
        <input v-model="form.backend.custom_path" placeholder="/custom_path or empty for app backend path" />
      </label>
    </template>

    <template v-else>
      <div class="field">
        <span>Discovered gRPC Method</span>
        <div class="grpc-reflection-row">
          <select
            v-model="selectedGrpcReflectionPath"
            :disabled="!appGrpcEnabled || grpcReflectionLoading || !grpcReflectionAvailable"
            @change="applyGrpcReflectionOption(selectedGrpcReflectionPath)"
          >
            <option value="">
              {{
                !appGrpcEnabled
                  ? "gRPC Port disabled for app"
                  : grpcReflectionLoading
                    ? "Loading reflection..."
                    : grpcReflectionAvailable
                      ? "Select method"
                      : "No reflection methods"
              }}
            </option>
            <option v-for="option in grpcReflectionOptions" :key="option.path" :value="option.path">
              {{ grpcReflectionOptionLabel(option) }}
            </option>
          </select>
          <button class="secondary-button" type="button" :disabled="!appGrpcEnabled || grpcReflectionLoading" @click="loadGrpcReflectionOptions">
            {{ grpcReflectionLoading ? "Loading..." : "Refresh" }}
          </button>
        </div>
        <span v-if="grpcReflectionDisabledReason" class="muted">{{ grpcReflectionDisabledReason }}</span>
        <span v-if="grpcReflectionError" class="muted">{{ grpcReflectionError }}</span>
      </div>
      <label class="field">
        <span>gRPC Service</span>
        <input v-model="form.grpc.service" placeholder="package.Service" required />
      </label>
      <label class="field">
        <span>gRPC Method</span>
        <input v-model="form.grpc.method" placeholder="Method" required />
      </label>
      <label class="field">
        <span>gRPC Path</span>
        <input v-model="form.grpc.path" placeholder="/package.Service/Method" required />
      </label>
    </template>

    <div class="field">
      <span>Auth</span>
      <AuthEditor v-model="form.auth" :jwt-kid-options="jwtKidOptions" />
    </div>

    <div class="actions">
      <button class="primary-button" type="submit" :disabled="saving">
        {{ saving ? "Saving..." : "Save" }}
      </button>
      <button class="secondary-button" type="button" :disabled="saving" @click="router.back()">Cancel</button>
    </div>
  </form>
</template>
