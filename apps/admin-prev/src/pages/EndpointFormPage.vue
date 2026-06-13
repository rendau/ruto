<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  createEndpoint,
  getApp,
  getAppInterpolate,
  getEndpoint,
  getEndpointInherited,
  getEndpointInterpolate,
  getRoot,
  getRootJwtKidsByUrls,
  updateEndpoint
} from "../lib/api";
import AuthEditor from "../components/AuthEditor.vue";
import LoggingEditor from "../components/LoggingEditor.vue";
import VariableEditor from "../components/VariableEditor.vue";
import VariableInput from "../components/VariableInput.vue";
import { emptyLogging, hasDuplicateVariableKeys, keyValueLinesToRecord, normalizeAuth, normalizeLogging, normalizeVariables, recordToKeyValueLines } from "../lib/forms";
import { notifyError, notifySuccess } from "../lib/notify";
import type { EndpointMain, EndpointType, Variable } from "../types/api";

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
const headersText = ref("");
const queryParamsText = ref("");
const effectiveVariables = ref<Variable[]>([]);
const hydratingVariables = ref(false);
let variablesRequestSeq = 0;

const form = ref<EndpointMain>({
  id: "",
  app_id: appIdFromRoute.value,
  active: true,
  exclude_from_metrics: false,
  http: {
    method: "GET",
    path: ""
  },
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
  logging: { ...emptyLogging },
  type: "http",
  grpc: {
    service: "",
    method: "",
    path: ""
  },
  variables: []
});
const includeInMetrics = computed({
  get: () => !form.value.exclude_from_metrics,
  set: (value: boolean) => {
    form.value.exclude_from_metrics = !value;
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
function normalizeLoadedEndpoint(item: EndpointMain): EndpointMain {
  const endpointType: EndpointType = item.type === "grpc" ? "grpc" : "http";
  return {
    ...item,
    type: endpointType,
    exclude_from_metrics: Boolean(item.exclude_from_metrics),
    http: {
      method: item.http?.method || "GET",
      path: item.http?.path || ""
    },
    backend: {
      custom_path: item.backend?.custom_path || "",
      headers: item.backend?.headers || {},
      query_params: item.backend?.query_params || {}
    },
    auth: normalizeAuth(item.auth),
    logging: normalizeLogging(item.logging),
    grpc: {
      service: item.grpc?.service || "",
      method: item.grpc?.method || "",
      path: item.grpc?.path || ""
    },
    variables: item.variables || []
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
      form.value.http.method = prefillMethodFromQuery.value;
    }
    if (prefillPathFromQuery.value) {
      form.value.http.path = prefillPathFromQuery.value;
    }
  }
}

async function loadAppName() {
  if (!form.value.app_id) {
    appName.value = "";
    appGrpcEnabled.value = false;
    return;
  }
  try {
    const app = await getApp(form.value.app_id);
    appName.value = app.name;
    appGrpcEnabled.value = Boolean((app.backend.grpc_url || "").trim());
  } catch {
    appName.value = "";
    appGrpcEnabled.value = false;
  }
}

async function load() {
  if (!isEdit.value) {
    applyPrefillFromQuery();
    await loadAppName();
    await refreshEffectiveVariables();
    return;
  }
  loading.value = true;
  errorMessage.value = "";
  try {
    hydratingVariables.value = true;
    form.value = normalizeLoadedEndpoint(await getEndpoint(endpointId.value));
    headersText.value = recordToKeyValueLines(form.value.backend.headers);
    queryParamsText.value = recordToKeyValueLines(form.value.backend.query_params);
    await loadAppName();
    await refreshEffectiveVariables();
    hydratingVariables.value = false;
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load endpoint";
  } finally {
    hydratingVariables.value = false;
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
    logging: normalizeLogging(form.value.logging),
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
    payload.http = {
      method: "",
      path: ""
    };
    payload.backend.custom_path = "";
    payload.backend.query_params = {};
    return payload;
  }

  payload.http = {
    method: (payload.http?.method || "").trim().toUpperCase(),
    path: (payload.http?.path || "").trim()
  };
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
  if (hasDuplicateVariableKeys(form.value.variables)) {
    errorMessage.value = "Variable keys must be unique";
    notifyError(errorMessage.value);
    saving.value = false;
    return;
  }
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

async function refreshEffectiveVariables() {
  if (!form.value.app_id) {
    effectiveVariables.value = [];
    return;
  }
  const requestId = ++variablesRequestSeq;
  try {
    const rep = isEdit.value && endpointId.value
      ? await getEndpointInterpolate({
        id: endpointId.value,
        app_id: form.value.app_id,
        variables: form.value.variables || []
      }).catch(async () =>
        getEndpointInherited({
          id: endpointId.value,
          app_id: form.value.app_id,
          variables: form.value.variables || []
        })
      )
      : await getAppInterpolate({
        id: form.value.app_id,
        variables: form.value.variables || []
      });
    if (requestId === variablesRequestSeq) {
      effectiveVariables.value = normalizeVariables(rep.variables);
    }
  } catch {
    if (requestId === variablesRequestSeq) {
      effectiveVariables.value = [];
    }
  }
}

watch(
  () => form.value.variables,
  () => {
    if (hydratingVariables.value) {
      return;
    }
    void refreshEffectiveVariables();
  },
  { deep: true }
);

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
    <div class="switches-row">
      <n-switch v-model:value="form.active">
        <template #checked>Active</template>
        <template #unchecked>Inactive</template>
      </n-switch>
      <n-switch v-model:value="includeInMetrics">
        <template #checked>Metrics</template>
        <template #unchecked>Metrics</template>
      </n-switch>
    </div>
    <div v-if="protocolOptions.length > 1" class="field">
      <span>Protocol</span>
      <n-tabs v-model:value="form.type" class="form-protocol-tabs" type="line" size="small" animated>
        <n-tab-pane v-for="option in protocolOptions" :key="option.value" :name="option.value" :tab="option.label" display-directive="show" />
      </n-tabs>
    </div>

    <template v-if="form.type === 'http'">
      <label class="field">
        <span>Method</span>
        <n-select v-model:value="form.http.method" :options="endpointMethodOptions.map((method) => ({ label: method, value: method }))" />
      </label>
      <label class="field">
        <span>Path</span>
        <n-input v-model:value="form.http.path" placeholder="/path or empty for app root" />
      </label>
      <label class="field">
        <span>Custom Backend Path</span>
        <n-input v-model:value="form.backend.custom_path" placeholder="/custom_path or empty for app backend path" />
      </label>
      <label class="field">
        <span>Backend Headers</span>
        <VariableInput v-model="headersText" :variables="effectiveVariables" type="textarea" :autosize="{ minRows: 4 }" placeholder="X-Service-Token: {{token}}" />
      </label>
      <label class="field">
        <span>Backend Query Params</span>
        <VariableInput v-model="queryParamsText" :variables="effectiveVariables" type="textarea" :autosize="{ minRows: 4 }" placeholder="tenant={{tenant}}" />
      </label>
    </template>

    <template v-else>
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
        <VariableInput v-model="headersText" :variables="effectiveVariables" type="textarea" :autosize="{ minRows: 4 }" placeholder="X-Service-Token: {{token}}" />
      </label>
    </template>

    <div class="field">
      <span>Variables</span>
      <VariableEditor v-model="form.variables" :available-variables="effectiveVariables" />
    </div>

    <div class="field">
      <span>Auth</span>
      <AuthEditor v-model="form.auth" :jwt-kid-options="jwtKidOptions" :variable-options="effectiveVariables" />
    </div>

    <div class="field">
      <span>Request logging</span>
      <LoggingEditor v-model="form.logging" />
    </div>

    <div class="actions" style="margin-top: 0.6rem;">
      <n-button type="primary" attr-type="submit" :loading="saving">
        {{ saving ? "Saving..." : "Save" }}
      </n-button>
      <n-button secondary :disabled="saving" @click="router.back()">Cancel</n-button>
    </div>
  </form>
</template>

<style scoped>
.switches-row {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  width: 100%;
  margin-top: 8px;
}
</style>
