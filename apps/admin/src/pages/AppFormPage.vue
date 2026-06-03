<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createApp, getApp, getAppSwaggerUrlByBackendUrl, getAppVariablesEffective, getRoot, getRootJwtKidsByUrls, updateApp } from "../lib/api";
import AuthEditor from "../components/AuthEditor.vue";
import VariableEditor from "../components/VariableEditor.vue";
import VariableInput from "../components/VariableInput.vue";
import { hasDuplicateVariableKeys, keyValueLinesToRecord, normalizeAuth, recordToKeyValueLines } from "../lib/forms";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppMain, Variable } from "../types/api";
import { useAppsStore } from "../stores/apps";

const route = useRoute();
const router = useRouter();
const appsStore = useAppsStore();

const isEdit = computed(() => typeof route.params.id === "string" && route.params.id.length > 0);
const entityId = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));

const loading = ref(false);
const saving = ref(false);
const errorMessage = ref("");
const jwtKidOptions = ref<string[]>([]);
const discoveringSwagger = ref(false);
const autoDetectedSwaggerUrl = ref("");
const headersText = ref("");
const queryParamsText = ref("");
const effectiveVariables = ref<Variable[]>([]);
let discoverTimer: ReturnType<typeof setTimeout> | null = null;
let discoverRequestSeq = 0;
let variablesRequestSeq = 0;

const form = ref<AppMain>({
  id: "",
  active: true,
  path_prefix: "",
  name: "",
  backend: {
    url: "",
    swagger_url: "",
    grpc_port: 0,
    headers: {},
    query_params: {}
  },
  auth: {
    enabled: true,
    mode: "extend",
    methods: []
  },
  variables: []
});

async function load() {
  if (!isEdit.value) {
    return;
  }

  loading.value = true;
  errorMessage.value = "";
  try {
    const item = await getApp(entityId.value);
    form.value = {
      ...item,
      backend: {
        ...item.backend,
        grpc_port: Number(item.backend.grpc_port || 0),
        headers: item.backend.headers || {},
        query_params: item.backend.query_params || {}
      },
      auth: normalizeAuth(item.auth),
      variables: item.variables || []
    };
    headersText.value = recordToKeyValueLines(form.value.backend.headers);
    queryParamsText.value = recordToKeyValueLines(form.value.backend.query_params);
    await refreshEffectiveVariables();
    if (!form.value.backend.swagger_url.trim()) {
      void discoverSwaggerUrl(form.value.backend.url);
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load app";
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

async function submit() {
  saving.value = true;
  errorMessage.value = "";
  if (hasDuplicateVariableKeys(form.value.variables)) {
    errorMessage.value = "Variable keys must be unique";
    notifyError(errorMessage.value);
    saving.value = false;
    return;
  }
  const payload: AppMain = {
    ...form.value,
    backend: {
      ...form.value.backend,
      grpc_port: Number(form.value.backend.grpc_port || 0),
      headers: keyValueLinesToRecord(headersText.value),
      query_params: keyValueLinesToRecord(queryParamsText.value)
    }
  };
  try {
    if (isEdit.value) {
      await updateApp(payload);
      await appsStore.loadMenuApps();
      notifySuccess("Application updated");
      await router.push({ name: "app-details", params: { id: payload.id } });
      return;
    }

    const created = await createApp(payload);
    await appsStore.loadMenuApps();
    notifySuccess("Application created");
    await router.push({ name: "app-details", params: { id: created.id } });
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to save app";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}

function onSwaggerUrlInput() {
  if (form.value.backend.swagger_url.trim() !== autoDetectedSwaggerUrl.value) {
    autoDetectedSwaggerUrl.value = "";
  }
}

async function discoverSwaggerUrl(backendUrl: string) {
  const normalizedBackendUrl = backendUrl.trim();
  if (!normalizedBackendUrl) {
    discoveringSwagger.value = false;
    return;
  }

  const requestId = ++discoverRequestSeq;
  discoveringSwagger.value = true;

  try {
    const rep = await getAppSwaggerUrlByBackendUrl({ backend_url: normalizedBackendUrl });
    if (requestId != discoverRequestSeq) {
      return;
    }

    const foundSwaggerUrl = (rep.swagger_url || "").trim();
    if (!foundSwaggerUrl) {
      return;
    }

    const currentSwaggerUrl = form.value.backend.swagger_url.trim();
    if (currentSwaggerUrl && currentSwaggerUrl !== autoDetectedSwaggerUrl.value) {
      return;
    }

    form.value.backend.swagger_url = foundSwaggerUrl;
    autoDetectedSwaggerUrl.value = foundSwaggerUrl;
  } catch {
    // user can still input swagger URL manually
  } finally {
    if (requestId == discoverRequestSeq) {
      discoveringSwagger.value = false;
    }
  }
}

function queueSwaggerDiscovery() {
  if (isEdit.value) {
    return;
  }
  if (discoverTimer) {
    clearTimeout(discoverTimer);
  }
  discoverTimer = setTimeout(() => {
    void discoverSwaggerUrl(form.value.backend.url);
  }, 600);
}

function triggerSwaggerDiscoveryNow() {
  if (isEdit.value) {
    return;
  }
  if (discoverTimer) {
    clearTimeout(discoverTimer);
    discoverTimer = null;
  }
  void discoverSwaggerUrl(form.value.backend.url);
}

async function refreshEffectiveVariables() {
  const requestId = ++variablesRequestSeq;
  try {
    const rep = await getAppVariablesEffective({
      id: isEdit.value ? entityId.value : "",
      variables: form.value.variables || []
    });
    if (requestId === variablesRequestSeq) {
      effectiveVariables.value = rep.variables || [];
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
    void refreshEffectiveVariables();
  },
  { deep: true }
);

onMounted(() => {
  void Promise.all([load(), loadJwtKidOptions(), refreshEffectiveVariables()]);
});

onBeforeUnmount(() => {
  if (discoverTimer) {
    clearTimeout(discoverTimer);
  }
  discoverRequestSeq++;
});
</script>

<template>
  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submit">
    <n-switch v-model:value="form.active">
      <template #checked>Active</template>
      <template #unchecked>Inactive</template>
    </n-switch>
    <label class="field">
      <span>Name</span>
      <n-input v-model:value="form.name" required />
    </label>
    <label class="field">
      <span>Path Prefix</span>
      <n-input v-model:value="form.path_prefix" placeholder="/example" required />
    </label>
    <label class="field">
      <span>Backend URL</span>
        <n-input
          v-model:value="form.backend.url"
          placeholder="https://example.com"
          required
          @input="queueSwaggerDiscovery"
          @blur="triggerSwaggerDiscoveryNow"
      />
    </label>
    <label class="field">
      <span>Swagger URL</span>
      <div class="input-with-indicator">
        <n-input
          v-model:value="form.backend.swagger_url"
          class="swagger-input"
          placeholder="https://example.com/swagger.json"
          @input="onSwaggerUrlInput"
        />
        <span v-if="discoveringSwagger" class="inline-spinner" role="status" aria-label="Searching swagger URL" />
      </div>
    </label>
    <label class="field compact">
      <span>gRPC Port</span>
      <n-input-number v-model:value="form.backend.grpc_port" :min="0" :max="65535" placeholder="0" />
    </label>
    <label class="field">
      <span>Backend Headers</span>
      <VariableInput v-model="headersText" :variables="effectiveVariables" type="textarea" :autosize="{ minRows: 4 }" placeholder="X-Service-Token: {{token}}" />
    </label>
    <label class="field">
      <span>Backend Query Params</span>
      <VariableInput v-model="queryParamsText" :variables="effectiveVariables" type="textarea" :autosize="{ minRows: 4 }" placeholder="tenant={{tenant}}" />
    </label>
    <div class="field">
      <span>Variables</span>
      <VariableEditor v-model="form.variables" :available-variables="effectiveVariables" />
    </div>
    <div class="field">
      <span>Auth</span>
      <AuthEditor v-model="form.auth" :jwt-kid-options="jwtKidOptions" :variable-options="effectiveVariables" />
    </div>

    <div class="actions">
      <n-button type="primary" attr-type="submit" :loading="saving">
        {{ saving ? "Saving..." : "Save" }}
      </n-button>
      <n-button secondary :disabled="saving" @click="router.back()">Cancel</n-button>
    </div>
  </form>
</template>

<style scoped>
.input-with-indicator {
  position: relative;
}

.inline-spinner {
  position: absolute;
  top: 50%;
  right: 12px;
  transform: translateY(-50%);
  width: 14px;
  height: 14px;
  border-radius: 999px;
  border: 2px solid #7c8ba5;
  border-top-color: #dce7f8;
  animation: spin 0.7s linear infinite;
  pointer-events: none;
}

.swagger-input {
  padding-right: 36px;
}

@keyframes spin {
  to {
    transform: translateY(-50%) rotate(360deg);
  }
}
</style>
