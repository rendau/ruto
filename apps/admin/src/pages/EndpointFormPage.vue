<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createEndpoint, getApp, getAppGrpcReflectionEndpoints, getEndpoint, getRoot, getRootJwtKidsByUrls, updateEndpoint } from "../lib/api";
import AuthEditor from "../components/AuthEditor.vue";
import { keyValueLinesToRecord, normalizeAuth, recordToKeyValueLines } from "../lib/forms";
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
const headersText = ref("");
const queryParamsText = ref("");

const form = ref<EndpointMain>({
  id: "",
  app_id: appIdFromRoute.value,
  active: true,
  method: "GET",
  path: "",
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
      custom_path: item.backend?.custom_path || "",
      headers: item.backend?.headers || {},
      query_params: item.backend?.query_params || {}
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
    headersText.value = recordToKeyValueLines(form.value.backend.headers);
    queryParamsText.value = recordToKeyValueLines(form.value.backend.query_params);
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
      custom_path: form.value.backend?.custom_path || "",
      headers: keyValueLinesToRecord(headersText.value),
      query_params: keyValueLinesToRecord(queryParamsText.value)
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
    payload.backend.query_params = {};
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
  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submit">
    <div class="field">
      <span>Application</span>
      <div class="field-readonly">{{ appDisplayName }}</div>
    </div>
    <n-switch v-model:value="form.active">
      <template #checked>Active</template>
      <template #unchecked>Inactive</template>
    </n-switch>
    <div v-if="protocolOptions.length > 1" class="field">
      <span>Protocol</span>
      <n-tabs v-model:value="form.type" class="form-protocol-tabs" type="line" size="small" animated>
        <n-tab-pane v-for="option in protocolOptions" :key="option.value" :name="option.value" :tab="option.label" display-directive="show" />
      </n-tabs>
    </div>

    <template v-if="form.type === 'http'">
      <label class="field">
        <span>Method</span>
        <n-select v-model:value="form.method" :options="endpointMethodOptions.map((method) => ({ label: method, value: method }))" />
      </label>
      <label class="field">
        <span>Path</span>
        <n-input v-model:value="form.path" placeholder="/path or empty for app root" />
      </label>
      <label class="field">
        <span>Custom Backend Path</span>
        <n-input v-model:value="form.backend.custom_path" placeholder="/custom_path or empty for app backend path" />
      </label>
      <label class="field">
        <span>Backend Headers</span>
        <n-input v-model:value="headersText" type="textarea" :autosize="{ minRows: 4 }" placeholder="X-Service-Token: secret" />
      </label>
      <label class="field">
        <span>Backend Query Params</span>
        <n-input v-model:value="queryParamsText" type="textarea" :autosize="{ minRows: 4 }" placeholder="tenant=acme" />
      </label>
    </template>

    <template v-else>
      <div class="field">
        <span>Discovered gRPC Method</span>
        <div class="grpc-reflection-row">
          <n-select
            v-model:value="selectedGrpcReflectionPath"
            :disabled="!appGrpcEnabled || grpcReflectionLoading || !grpcReflectionAvailable"
            :placeholder="
                !appGrpcEnabled
                  ? 'gRPC Port disabled for app'
                  : grpcReflectionLoading
                    ? 'Loading reflection...'
                    : grpcReflectionAvailable
                      ? 'Select method'
                      : 'No reflection methods'
              "
            :options="grpcReflectionOptions.map((option) => ({ label: grpcReflectionOptionLabel(option), value: option.path }))"
            clearable
            @update:value="(value: string | null) => applyGrpcReflectionOption(value || '')"
          />
          <n-button secondary :disabled="!appGrpcEnabled || grpcReflectionLoading" :loading="grpcReflectionLoading" @click="loadGrpcReflectionOptions">
            {{ grpcReflectionLoading ? "Loading..." : "Refresh" }}
          </n-button>
        </div>
        <span v-if="grpcReflectionDisabledReason" class="muted">{{ grpcReflectionDisabledReason }}</span>
        <span v-if="grpcReflectionError" class="muted">{{ grpcReflectionError }}</span>
      </div>
      <label class="field">
        <span>gRPC Service</span>
        <n-input v-model:value="form.grpc.service" placeholder="package.Service" required />
      </label>
      <label class="field">
        <span>gRPC Method</span>
        <n-input v-model:value="form.grpc.method" placeholder="Method" required />
      </label>
      <label class="field">
        <span>gRPC Path</span>
        <n-input v-model:value="form.grpc.path" placeholder="/package.Service/Method" required />
      </label>
      <label class="field">
        <span>Backend Headers</span>
        <n-input v-model:value="headersText" type="textarea" :autosize="{ minRows: 4 }" placeholder="X-Service-Token: secret" />
      </label>
    </template>

    <div class="field">
      <span>Auth</span>
      <AuthEditor v-model="form.auth" :jwt-kid-options="jwtKidOptions" />
    </div>

    <div class="actions">
      <n-button type="primary" attr-type="submit" :loading="saving">
        {{ saving ? "Saving..." : "Save" }}
      </n-button>
      <n-button secondary :disabled="saving" @click="router.back()">Cancel</n-button>
    </div>
  </form>
</template>
